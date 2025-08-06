# Project TODO List

This file tracks the remaining tasks for the Orthodox Pilgrimage website project.

## ✅ Completed
- [x] ~~Set up the Go project structure~~
- [x] ~~Define the database schema in SQL~~
- [x] ~~Write initial queries and use `sqlc` to generate Go models and query code~~
- [x] ~~Create a simple command-line script or tool to seed the database with initial data~~
- [x] ~~Set up a basic Go web server using `net/http`~~
- [x] ~~Create an API endpoint that returns a list of churches as JSON~~
- [x] ~~Build the initial frontend page with a full-screen Leaflet.js map that fetches and displays church locations from the API endpoint~~

## Phase 2: Core Map & API Endpoint
- [ ] Add geographic boundary filtering to the `/api/v1/churches` endpoint.

## Phase 3: HTMX Interactivity & UI

- [ ] Create a server endpoint that renders an HTML fragment for a single church's details.
- [ ] Use HTMX so that clicking a map pin fetches and displays this HTML fragment.
- [ ] Implement the URL update for bookmarking when a church is selected (using `hx-push-url`).
- [ ] Implement the city search functionality using an HTMX-powered form.
- [ ] Develop the overall layout and styling.

## Phase 4: Deployment

- [ ] Containerize the application using a `Dockerfile`.
- [ ] Prepare for deployment to a hosting service.