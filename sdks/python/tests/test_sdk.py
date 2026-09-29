"""Run against the ephemeral Go server started by sdk_test.go."""

import sys
import unittest

import httpx

from filament import (
    AsyncFilament,
    Filament,
    IngestionV1PaginationRequest,
    IngestionV1WorkerConfiguration,
    IngestionV1WorkerResources,
)
from filament.core.api_error import ApiError

BASE_URL = sys.argv.pop(1)


class TestSDK(unittest.TestCase):
    def setUp(self):
        self.http = httpx.Client()
        self.addCleanup(self.http.close)
        self.client = Filament(
            base_url=BASE_URL, token="sdk-test", httpx_client=self.http
        )

    def test_pipeline_round_trip(self):
        created = self.client.pipeline.create(
            name="sdk-round-trip",
            worker_configuration=IngestionV1WorkerConfiguration(
                node_selector={"sdk-test": "true"},
                resources=IngestionV1WorkerResources(requests={"cpu": "250m"}),
            ),
        ).pipeline
        self.assertIsNotNone(created)
        self.assertTrue(created.id)
        self.assertGreater(int(created.created_at), 0)
        self.assertEqual(created.worker_configuration.node_selector, {"sdk-test": "true"})
        self.assertEqual(created.worker_configuration.resources.requests["cpu"], "250m")
        self.assertEqual(self.client.pipeline.get(id=created.id).pipeline.name, "sdk-round-trip")

        listed = self.client.pipeline.list(
            search="sdk-round-trip", pagination=IngestionV1PaginationRequest(page_size=1)
        )
        self.assertEqual([p.id for p in listed.pipelines], [created.id])
        self.assertEqual(int(listed.pagination.total), 1)

        updated = self.client.pipeline.update(
            pipeline_id=created.id,
            name="sdk-updated",
            worker_configuration=created.worker_configuration,
        ).pipeline
        self.assertEqual(updated.name, "sdk-updated")
        self.client.pipeline.delete(id=created.id)
        self.assertFalse(self.client.pipeline.list(search="sdk-updated").pipelines)

    def test_public_auth_and_protected_errors(self):
        public = Filament(base_url=BASE_URL, httpx_client=self.http)
        self.assertEqual(public.auth.get_config(request={}).issuer, "https://identity.example.test")
        with self.assertRaises(ApiError) as raised:
            public.pipeline.list()
        self.assertEqual(raised.exception.status_code, 401)
        self.assertEqual(raised.exception.body["code"], "unauthenticated")

        with self.assertRaises(ApiError) as raised:
            self.client.pipeline.get(id="missing")
        self.assertEqual(raised.exception.status_code, 404)
        self.assertEqual(raised.exception.body["code"], "not_found")

    def test_callable_token(self):
        token = "invalid"
        client = Filament(base_url=BASE_URL, token=lambda: token, httpx_client=self.http)
        with self.assertRaises(ApiError):
            client.connector.list()
        token = "sdk-test"
        self.assertFalse(client.connector.list().connectors)

    def test_mutations_are_not_retried_by_default(self):
        attempts = []

        def unavailable(request):
            attempts.append(request)
            return httpx.Response(503, json={"code": "unavailable", "message": "retry later"})

        with httpx.Client(transport=httpx.MockTransport(unavailable)) as http:
            client = Filament(base_url=BASE_URL, token="sdk-test", httpx_client=http)
            with self.assertRaises(ApiError):
                client.pipeline.create(name="do-not-duplicate")
        self.assertEqual(len(attempts), 1)


class TestAsyncSDK(unittest.IsolatedAsyncioTestCase):
    async def test_async_round_trip(self):
        async with httpx.AsyncClient() as http:
            client = AsyncFilament(base_url=BASE_URL, token="sdk-test", httpx_client=http)
            created = (await client.pipeline.create(name="sdk-async")).pipeline
            loaded = (await client.pipeline.get(id=created.id)).pipeline
            self.assertEqual(loaded.name, "sdk-async")
            await client.pipeline.delete(id=created.id)


if __name__ == "__main__":
    unittest.main()
