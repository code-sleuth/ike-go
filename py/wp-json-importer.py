import asyncio
import aiohttp
import asyncpg
import json
import logging
import ssl
from urllib.parse import urlparse
import time
from dotenv import load_dotenv
import os

load_dotenv()

# Configure logging
logging.basicConfig(level=logging.INFO)

# URLs to fetch data from
urls = [
    "https://wsform.com/wp-json/wp/v2/knowledgebase",
]


async def create_pool():
    load_dotenv()
    return await asyncpg.create_pool(
        user=os.getenv("DB_USER"),
        password=os.getenv("DB_PASSWORD"),
        database=os.getenv("DB_NAME"),
        host=os.getenv("DB_HOST"),
        port=os.getenv("DB_PORT"),
        ssl="require",
    )


# Insert source information into the database
async def insert_source(pool, url):
    async with pool.acquire() as conn:
        parsed_url = urlparse(url)
        scheme, host, path, query = (
            parsed_url.scheme,
            parsed_url.netloc,
            parsed_url.path,
            parsed_url.query,
        )
        result = await conn.fetchrow(
            "INSERT INTO sources (raw_url, scheme, host, path, query, format) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
            url,
            scheme,
            host,
            path,
            query,
            "json",
        )
        return result["id"]


# Insert content information into the database
async def insert_content(pool, source_id, status_code, headers, body):
    async with pool.acquire() as conn:
        result = await conn.fetchrow(
            "INSERT INTO downloads (source_id, status_code, headers, body) VALUES ($1, $2, $3, $4) RETURNING id",
            source_id,
            status_code,
            json.dumps(headers),
            body,
        )
        return result["id"]


async def fetch_and_process_post(session, base_url, post_id, pool):
    url = f"{base_url}/{post_id}"  # Construct the URL dynamically
    async with session.get(url) as response:
        if response.status == 200:
            try:
                data = await response.json()
                source_id = await insert_source(pool, url)
                await insert_content(
                    pool,
                    source_id,
                    response.status,
                    dict(response.headers),
                    json.dumps(data),
                )
                logging.info(f"Processed post from {url}")
            except aiohttp.client_exceptions.ContentTypeError:
                logging.error(f"Error decoding JSON from {url}: unexpected mimetype")
        else:
            logging.error(f"Error fetching post {url}: HTTP status {response.status}")


async def get_post_ids_and_process(session, full_url, pool):
    page = 1
    while True:
        url = f"{full_url}?page={page}&per_page=100"
        async with session.get(url) as response:
            if response.status == 400:
                break
            try:
                data = await response.json()
                tasks = [
                    fetch_and_process_post(session, full_url, post["id"], pool)
                    for post in data
                ]  # Pass full_url here
                await asyncio.gather(*tasks)
                page += 1
            except aiohttp.client_exceptions.ContentTypeError:
                logging.error(f"Error decoding JSON from {url}: unexpected mimetype")


# Main coroutine to manage the workflow
async def main():
    # Create a connection pool
    pool = await create_pool()
    ssl_context = ssl.create_default_context()
    ssl_context.check_hostname = False
    ssl_context.verify_mode = ssl.CERT_NONE

    async with aiohttp.ClientSession(
        connector=aiohttp.TCPConnector(ssl=ssl_context)
    ) as session:
        tasks = [get_post_ids_and_process(session, url, pool) for url in urls]
        await asyncio.gather(*tasks)
    # Close the connection pool
    await pool.close()


# Measure execution time
start_time = time.time()
asyncio.run(main())
end_time = time.time()
print(f"Total execution time: {end_time - start_time} seconds")