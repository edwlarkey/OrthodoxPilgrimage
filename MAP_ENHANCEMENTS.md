# Map Enhancements Plan: Clustering, Performance & Filtering

This document outlines the technical strategy for enhancing the Orthodox Pilgrimage map with marker clustering, performance optimizations, and advanced data filtering.

## 1. Data Schema & Model Updates (Normalization)

To support robust filtering and data integrity, we are moving from simple strings to normalized lookup tables.

### 1.1 Jurisdictions Table
A new `jurisdictions` table will centralize jurisdiction data and their associated traditions.
- **Columns:**
    - `id`: INTEGER PK
    - `name`: TEXT UNIQUE (e.g., "Greek Orthodox Archdiocese", "Roman Catholic")
    - `tradition`: TEXT (e.g., "Orthodox", "Roman Catholic", "Oriental Orthodox")
    - `pin_color`: TEXT (Hex code for the map pin, e.g., `#530c38` for Orthodox, `#d4af37` for Catholic)
- **Church Table Update:** Replace `jurisdiction` string column with `jurisdiction_id` foreign key.

### 1.2 Relic Types Table
A new `relic_types` table will categorize the nature of the holy items.
- **Columns:**
    - `id`: INTEGER PK
    - `name`: TEXT UNIQUE (e.g., "Major", "Fragment", "Secondary", "Other")
    - `sort_order`: INTEGER (Ensures "Major" appears first in filters/lists)
- **Relics Table Update:** Add `relic_type_id` foreign key.

### 1.3 Migration & Seeding Strategy
- **Migration:** Create `jurisdictions` and `relic_types` tables. Migrate existing string data into these tables and update foreign keys.
- **Data File:** Update `internal/app/data/data.json` to include these new structured relationships.
- **Seeder:** Update `internal/app/seed.go` to handle lookup table insertion and mapping.

---

## 2. Frontend Map Enhancements (OpenLayers)

### 2.1 Marker Clustering
- **Implementation:** Use `ol/source/Cluster`.
- **Logic:** Group pins within a 40px radius.
- **UX:** 
    - Cluster icon displays the total count.
    - Clicking a cluster zooms the map to fit all contained markers (`view.fit`).

### 2.2 Dynamic Pin Styling
- **Logic:** The API will now include `tradition` and `pin_color` (sourced from the `jurisdictions` table) for each church.
- **Execution:** OpenLayers style function will dynamically select the pin color based on the church's jurisdiction data.

### 2.3 Performance Optimization
- **Style Caching:** Cache the generated SVG icons for each color to prevent redundant rendering during map interactions.
- **Client-Side Filtering:** The `VectorSource` will be filtered in-memory for instant feedback when users toggle filter options.

---

## 3. Filtering Interface

### 3.1 Filter Icon & Panel
To keep the map clean, filters will be hidden behind a **Filter Icon**.
- **Location:** Top-right or bottom-right map overlay.
- **Panel:** A sliding or modal "glassmorphism" panel containing:
    - **Tradition:** (e.g., [x] Orthodox [ ] Roman Catholic)
    - **Relic Importance:** (e.g., [ ] Major Only [x] All)
    - **Jurisdiction:** Multi-select dropdown.

### 3.2 Logic
- **HTMX:** Used to fetch the list of available jurisdictions/types if they change.
- **Vanilla JS:** Intercepts filter changes and applies `feature.setStyle(null)` or `vectorSource.removeFeature()` logic for instant map updates.

---

## 4. Implementation Roadmap

1.  **Database Migration:** Create new tables and update existing schemas.
2.  **API Update:** Modify `/api/v1/churches` to include tradition, pin color, and relic type metadata.
3.  **Data Update:** Refactor `data.json` and the Go seeder.
4.  **Map Core:** Implement OpenLayers Clustering and zoom-to-cluster logic.
5.  **Map Styling:** Implement dynamic pin coloring based on the new jurisdiction tradition.
6.  **Filter UI:** Build the interactive filter panel and connect it to the map source.
