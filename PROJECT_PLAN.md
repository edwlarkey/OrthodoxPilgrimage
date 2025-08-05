# Orthodox Pilgrimage Website - Project Plan

This document outlines the plan for creating a website to help Orthodox Christians plan pilgrimages to churches with saints' relics.

## 1. Vision

To create a resource that maps the locations of Orthodox churches containing relics of saints available for public veneration, making it easy for people to plan pilgrimages.

## 2. Core Features (MVP - Minimum Viable Product)

The initial version of the site will focus on the core functionality.

*   **Interactive Map:** A full-screen map that shows church locations. The map will dynamically load new church markers as the user pans and zooms.
*   **Church Details:** Clicking a map pin will display the church's details. The page URL will update to reflect the selected church, allowing it to be bookmarked and shared (e.g., `/churches/123`).
*   **City Search:** A search bar allowing users to find and jump to a specific city on the map.
*   **Responsive Design:** The layout will be fully functional and usable on desktop, tablet, and mobile browsers.
*   **No User Accounts:** The initial release will be purely informational and will not require user login.

## 3. Technology Stack

The project will be built with the following technologies:

*   **Backend:** Go
    *   **Web Server:** Standard Library `net/http`.
    *   **Database:** SQLite.
    *   **Query Generation:** `sqlc` to generate type-safe Go code from SQL queries.
*   **Frontend:**
    *   **HTML Templating:** Go's `html/template` package.
    *   **Interactivity:** HTMX to handle user actions without full page reloads.
    *   **Mapping Library:** OpenStreetMap with the Leaflet.js library.
    *   **Styling:** Simple, clean custom CSS. Minimal Javascript.
*   **API Style:** HATEOAS (Hypermedia as the Engine of Application State). The server will respond with HTML fragments containing hypermedia controls that guide user interaction.

## 4. Data Strategy

*   **Data Source:** Initial data will be populated and managed by an admin. There will be no public submission feature in the MVP.
*   **Address Handling:** We will use a hybrid approach to support international addresses while allowing for robust searching. An admin will provide a single address string, which will be geocoded on the backend to extract coordinates and structured location data for storage.
*   **Data Model (Initial):**

    ### `churches` table
    | Column | Type | Notes |
    |---|---|---|
    | `id` | INTEGER | Primary Key |
    | `name` | TEXT | The full name of the church |
    | `address_text` | TEXT | The full, formatted address for display (e.g., "123 Main St, Anytown, USA") |
    | `city` | TEXT | For searching and filtering |
    | `state_province` | TEXT | For searching and filtering (e.g., "California", "Attica") |
    | `country_code` | TEXT | Two-letter ISO country code (e.g., "US", "GR") for filtering |
    | `latitude` | REAL | GPS coordinate for map pin location |
    | `longitude` | REAL | GPS coordinate for map pin location |
    | `jurisdiction` | TEXT | e.g., "Greek Orthodox Archdiocese of America", "Antiochian Orthodox..." |
    | `website` | TEXT | Official website URL (optional) |
    | `description` | TEXT | Additional notes about the church or relics (optional) |

    ### `saints` table
    | Column | Type | Notes |
    |---|---|---|
    | `id` | INTEGER | Primary Key |
    | `name` | TEXT | The name of the saint |
    | `feast_day` | TEXT | e.g., "December 6" (optional) |
    | `description` | TEXT | A short bio or description of the saint (optional) |

    ### `relics` table (Join Table)
    | Column | Type | Notes |
    |---|---|---|
    | `church_id` | INTEGER | Foreign Key to `churches.id` |
    | `saint_id` | INTEGER | Foreign Key to `saints.id` |
    | `description` | TEXT | Description of this specific relic (e.g., "First-class relic") |


## 5. Project Roadmap

This is a high-level plan for development.

*   **Phase 1: Backend Setup & Data Modeling**
    *   Set up the Go project structure (`/cmd`, `/internal`, etc.).
    *   Define the database schema in SQL (`schema.sql`).
    *   Write initial queries (`queries.sql`) and use `sqlc` to generate Go models and query code.
    *   Create a simple command-line script or tool to seed the database with initial data (including geocoding).

*   **Phase 2: Core Map & API Endpoint**
    *   Set up a basic Go web server using `net/http`.
    *   Create an API endpoint that returns a list of churches (in a specific geographic boundary) as JSON.
    *   Build the initial frontend page with a full-screen Leaflet.js map that fetches and displays church locations from the API endpoint.

*   **Phase 3: HTMX Interactivity & UI**
    *   Create a server endpoint that renders an HTML fragment for a single church's details.
    *   Use HTMX so that clicking a map pin fetches and displays this HTML fragment.
    *   Implement the URL update for bookmarking when a church is selected (using `hx-push-url`).
    *   Implement the city search functionality using an HTMX-powered form.
    *   Develop the overall layout and styling.

*   **Phase 4: Deployment**
    *   Containerize the application using a `Dockerfile`.
    *   Prepare for deployment to a hosting service.

## 6. Future Features (Post-MVP)

*   **Admin Interface:** A web-based UI for admins to add/edit/delete data.
*   **User Accounts:** Allowing users to save favorite churches or plan itineraries.
*   **Crowdsourcing:** A system for users to submit new churches or suggest corrections, with an admin approval workflow.
*   **Advanced Filtering:** Filter churches by jurisdiction or saint.
*   **Detailed Saint Pages:** Pages with more information about each saint.
