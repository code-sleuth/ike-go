import asyncio
import asyncpg
import httpx
import json
from markdownify import markdownify as md
from datetime import datetime
import re
import os
from dotenv import load_dotenv
import pycountry
from langdetect import detect
import tiktoken
from dateutil.parser import parse
import cProfile
import pstats

import traceback
import time

load_dotenv()


# using custom db pools for more concurrent connections
async def connect_db():
    return await asyncpg.create_pool(
        user=os.getenv("DB_USER"),
        password=os.getenv("DB_PASSWORD"),
        database=os.getenv("DB_NAME"),
        host=os.getenv("DB_HOST"),
        port=os.getenv("DB_PORT"),
        ssl="require",
        # min_size=5,  # Minimum number of connection in the pool
        # max_size=10,  # Maximum number of connection in the pool
    )


# async def get_source(id, conn):
#     if id:
#         return await conn.fetchrow("SELECT * FROM sources WHERE id = $1", id)
#     else:
#         return await conn.fetchrow("SELECT * FROM sources ORDER BY id DESC LIMIT 1")


async def get_sources(conn, host=None):
    if host:
        return await conn.fetch("SELECT * FROM sources WHERE host = ANY($1)", [host])
    else:
        return await conn.fetch("SELECT * FROM sources")


async def get_download(source_id, conn):
    return await conn.fetchrow(
        "SELECT * FROM downloads WHERE source_id = $1", source_id
    )


async def embed_document(content, embedding_info):
    model = embedding_info["name"]

    headers = {
        "Content-Type": "application/json",
    }
    payload = {
        "input": content.replace("\n", " "),
        "model": model,
    }

    if "togethercomputer/" in model:
        endpoint_url = "https://api.together.xyz/v1/embeddings"
        headers["Authorization"] = f"Bearer {os.getenv('TOGETHER_API_KEY')}"
    elif "text-embedding" in model:
        endpoint_url = "https://api.openai.com/v1/embeddings"
        headers["Authorization"] = f"Bearer {os.getenv('OPENAI_API_KEY')}"
        payload["encoding_format"] = "float"
    else:
        raise ValueError("Unsupported model")

    async with httpx.AsyncClient() as client:
        response = await client.post(
            endpoint_url, headers=headers, data=json.dumps(payload)
        )
        response.raise_for_status()  # This will throw an exception for non-2xx responses
        data = response.json()
        vector = data["data"][0]["embedding"]
        dimension = len(vector)

    return vector, dimension


async def get_tags(source_url):
    async with httpx.AsyncClient() as client:
        response = await client.get(f"{source_url}?_embed")
        response.raise_for_status()
        data = response.json()

        terms = [
            term
            for sublist in data.get("_embedded", {}).get("wp:term", [])
            for term in sublist
        ]
        tags = [term["name"] for term in terms]

        return tags


def number_of_links(document):
    return len(re.findall(r"\[[^\]]*\]\([^\)]*\)", document))


async def process_document(download):
    body = json.loads(download[6])
    content = md(body["content"]["rendered"])
    modified_gmt = datetime.strptime(body["modified_gmt"], "%Y-%m-%dT%H:%M:%S")
    date_gmt = datetime.strptime(body["date_gmt"], "%Y-%m-%dT%H:%M:%S")

    natural_lang = None
    try:
        detected_lang = detect(content)
        if detected_lang:
            lang = pycountry.languages.get(alpha_2=detected_lang)
            natural_lang = lang.name.lower() if lang else None
    except Exception as e:
        print(f"Error detecting language: {e}")

    document = {
        "modified_at": parse(modified_gmt.isoformat()),
        "published_at": parse(date_gmt.isoformat()),
        "format": "md",
    }

    return document, content, natural_lang


def process_metadata(download, content):
    body = json.loads(download[6])
    metadata = {
        "document_title": md(body["title"]["rendered"]),
        "document_description": md(body["excerpt"]["rendered"]),
        "links_count": number_of_links(content),
        "canonical_url": body["link"],
    }

    return metadata


async def process_chunks(content, natural_lang, embedding_info):
    byte_size = len(content.encode("utf-8"))
    encoding_type = "cl100k_base"
    encoding = tiktoken.get_encoding(encoding_type)
    tokenized_content = encoding.encode(content)
    token_count = len(tokenized_content)

    if token_count > embedding_info["max_context"]:
        chunk_size = embedding_info["max_context"]
        chunks = [
            tokenized_content[i : i + chunk_size]
            for i in range(0, token_count, chunk_size)
        ]
        chunk_dicts = []
        for chunk_tokens in chunks:
            chunk = encoding.decode(chunk_tokens)
            embedding_vec, embedding_dim = await embed_document(chunk, embedding_info)
            chunk_dict = {
                "body": chunk,
                "byte_size": len(chunk.encode("utf-8")),
                "tokenizer": encoding_type,
                "token_count": len(chunk_tokens),
                "natural_lang": natural_lang,
                "embedding": {
                    "model": embedding_info["name"],
                    "vector": embedding_vec,
                    "dimension": embedding_dim,
                },
            }
            chunk_dicts.append(chunk_dict)
    else:
        embedding_vec, embedding_dim = await embed_document(content, embedding_info)
        chunk_dicts = [
            {
                "body": content,
                "byte_size": byte_size,
                "tokenizer": encoding_type,
                "token_count": token_count,
                "natural_lang": natural_lang,
                "embedding": {
                    "model": embedding_info["name"],
                    "vector": embedding_vec,
                    "dimension": embedding_dim,
                },
            }
        ]

    return chunk_dicts


async def insert_document(source_id, download_id, doc_data, embedding_data, conn):
    query = """
        INSERT INTO documents (source_id, download_id, format, min_chunk_size, max_chunk_size, published_at, modified_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (source_id) DO UPDATE SET
            download_id = EXCLUDED.download_id,
            format = EXCLUDED.format,
            min_chunk_size = EXCLUDED.min_chunk_size,
            max_chunk_size = EXCLUDED.max_chunk_size,
            published_at = EXCLUDED.published_at,
            modified_at = EXCLUDED.modified_at
        RETURNING id
    """
    return await conn.fetchval(
        query,
        source_id,
        download_id,
        doc_data["format"],
        212,
        embedding_data["max_context"],
        doc_data["published_at"],
        doc_data["modified_at"],
    )


async def insert_chunk(
    conn, document_id, chunk_data, left_chunk_id=None, right_chunk_id=None
):
    query = """
        INSERT INTO chunks (document_id, body, byte_size, tokenizer, token_count, natural_lang, left_chunk_id, right_chunk_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id
    """
    return await conn.fetchval(
        query,
        document_id,
        chunk_data["body"],
        chunk_data["byte_size"],
        chunk_data["tokenizer"],
        chunk_data["token_count"],
        chunk_data["natural_lang"],
        left_chunk_id,
        right_chunk_id,
    )


async def insert_embedding(conn, chunk_id, embedding_data):
    dimension = embedding_data["dimension"]

    column_name = f"embedding_{dimension}"
    query = f"""
        INSERT INTO embeddings (object_id, model, {column_name}, object_type)
        VALUES ($1, $2, $3, $4)
        RETURNING id
    """
    return await conn.fetchval(
        query,
        chunk_id,
        embedding_data["model"],
        str(embedding_data["vector"]),
        "chunk",
    )


async def insert_metadata(conn, document_id, metadata):
    inserted_meta_ids = []
    for key, value in metadata.items():
        query = """
            INSERT INTO document_meta (document_id, key, meta)
            VALUES ($1, $2, $3)
            RETURNING id
        """
        meta_id = await conn.fetchval(query, document_id, key, json.dumps(value))
        inserted_meta_ids.append(meta_id)
    return inserted_meta_ids


async def insert_tags(conn, document_id, tags):
    for tag in tags:
        # Check if the tag already exists and get its ID
        tag_id = await conn.fetchval("SELECT id FROM tags WHERE name = $1", tag)
        if not tag_id:
            # Insert new tag if it doesn't exist
            tag_id = await conn.fetchval(
                "INSERT INTO tags (name) VALUES ($1) RETURNING id", tag
            )

        # Link the document and the tag
        await conn.execute(
            "INSERT INTO document_tags (document_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
            document_id,
            tag_id,
        )


async def update_chunk_with_right_chunk_id(conn, chunk_id, right_chunk_id):
    query = """
        UPDATE chunks
        SET right_chunk_id = $1
        WHERE id = $2
    """
    await conn.execute(query, right_chunk_id, chunk_id)


async def process_source(source, pool):
    async with pool.acquire() as conn:
        try:
            start_time = time.time()

            # open ai is 8191
            embedding_info = {"name": "text-embedding-3-small", "max_context": 8191}

            download = await get_download(source["id"], conn)
            document, content, natural_lang = await process_document(download)

            document["max_chunk_size"] = embedding_info["max_context"]
            document["min_chunk_size"] = 212

            metadata = process_metadata(download, content)
            chunks = await process_chunks(content, natural_lang, embedding_info)

            doc_id = await insert_document(
                source["id"], download["id"], document, embedding_info, conn
            )

            previous_chunk_id = None
            for i, chunk in enumerate(chunks):
                left_chunk_id = previous_chunk_id
                right_chunk_id = None

                if i < len(chunks) - 1:
                    chunk_id = await insert_chunk(conn, doc_id, chunk, left_chunk_id)
                else:
                    chunk_id = await insert_chunk(
                        conn, doc_id, chunk, left_chunk_id, right_chunk_id
                    )

                if previous_chunk_id is not None:
                    await update_chunk_with_right_chunk_id(
                        conn, previous_chunk_id, chunk_id
                    )

                previous_chunk_id = chunk_id

                embedding_id = await insert_embedding(
                    conn, chunk_id, chunk["embedding"]
                )

            metadata_ids = await insert_metadata(conn, doc_id, metadata)

            end_time = time.time()

            print(
                f"Processed and inserted data for source {source['id']} in { end_time - start_time}"
            )
        except Exception as e:
            print(f"Error processing source {source['id']}: {e}")
            print(f"Exception type: {type(e)}")
            print(traceback.format_exc())


async def process_sources(conn, limit=None):
    sources = await get_sources(conn, ["wsform.com", "wpsimplepay.com"])
    tasks = []
    for i, source in enumerate(sources):
        if limit is not None and i >= limit:
            break
        tasks.append(process_source(source, conn))
    await asyncio.gather(*tasks)


async def main():
    start_time = time.time()
    conn = await connect_db()
    # we can add limit here for future testing
    await process_sources(conn)
    await conn.close()
    end_time = time.time()
    print(f"Total time: {end_time - start_time}")


if __name__ == "__main__":
    # profiler = cProfile.Profile()
    # profiler.enable()
    asyncio.run(main())
    # profiler.disable()
    # stats = pstats.Stats(profiler).sort_stats("cumtime")
    # stats.dump_stats("profile_results.pstats")