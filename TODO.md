# Project TODO List

This file tracks the remaining tasks for the Orthodox Pilgrimage website project.

## ✅ Completed

- [x] Set up the Go project structure
- [x] Define the database schema in SQL
- [x] Write initial queries and use `sqlc` to generate Go models and query code
- [x] Create a simple command-line script or tool to seed the database with initial data
- [x] Set up a basic Go web server using `net/http`
- [x] Create an API endpoint that returns a list of churches as JSON
- [x] Build the initial frontend page with a full-screen OpenLayers map (switched from Leaflet)
- [x] Create a server endpoint that renders an HTML fragment for a single church's details.
- [x] Use HTMX so that clicking a map pin fetches and displays this HTML fragment.
- [x] Implement the URL update for bookmarking when a church is selected (using `hx-push-url`).
- [x] Implement the city search functionality (Searching by Saint).
- [x] Develop the overall layout and styling.
- [x] **SEO Optimizations**:
    - [x] Dynamic page titles and meta descriptions for all pages.
    - [x] JSON-LD Structured Data (Schema.org) for Churches and Saints.
    - [x] Open Graph (Facebook) and Twitter Card support.
    - [x] Canonical URL tags.
    - [x] Automated Sitemap.xml generation on startup.
    - [x] Robots.txt with fly.dev indexing protection.
    - [x] HTMX dynamic title updates via `HX-Title`.

## Phase 2: Core Map & API Endpoint

- [x] Add geographic boundary filtering to the `/api/v1/churches` endpoint.


## SEO Improvements & Maintenance

- [ ] **Structured Data**: Add `BreadcrumbList` schema to Church and Saint pages.
- [x] **Directory Pages**: Create `/saints` and `/churches` listing pages for better crawlability.
- [x] **Performance**: Optimize map script loading (lazy-load or defer).
- [ ] **Assets**: Add a high-quality `og-image.jpg` to `internal/ui/static/`.
- [x] **Images**: Ensure all images have descriptive `alt` text support.

