# Book Manager
### A Restful App to manage your Books.

---

A small RESTful API service in for managing books.
Implemented using [Hexagonal Architecture](https://en.wikipedia.org/wiki/Hexagonal_architecture_(software)) 
with in-memory storage and optional enrichment from the [Open Library API](https://openlibrary.org/swagger/docs#/books/read_isbn_isbn__isbn__get).

---
### Features

- CRUD for books (create, list, read, delete)
- Optional external enrichment via ISBN (title, authors, year, cover URL)
- Clean separation of core domain and adapters and ports interface.
- Tests at repo, service, HTTP, and integration layers
- OpenAPI-first: API defined in openapi.yaml, server stubs generated with oapi-codegen

--- 
### Project Structure
```
cmd/api           – main entrypoint
internal/core     – domain models, service layer
internal/adapter  – adapters (in-memory repo, HTTP, open-library clients)
api               – generated OpenAPI types & server glue
```
---
### Requirements

- Go 1.22+
- Make
- oapi-codegen

---
### Running

```bash
go run ./cmd/api
```
Service listens on :8080 by default.

Once server started and ready, 
Run the sample request from `cmd/api/Requests.http`, run via IDE or use [cURL](https://curl.se/) command.

### Improvements (future extension)
- Must Have
  - Persistent storage (e.g., PostgreSQL, Redis)
  - Richer Open Library client (author lookups, editions)
- Nice to have
  - Caching of enrichment responses
  - Request validation via OpenAPI middleware