# Admin Interface Plan - Orthodox Pilgrimage

This document defines the architecture, security protocols, and feature set for the administrative backend.

## 1. Security & Authentication

### 1.1 Mandatory Multi-Factor Authentication (MFA)
Access to the admin interface requires two-step verification. Social authentication is explicitly excluded.
- **Factor 1:** Local credentials (Username/Password) hashed with Argon2id.
- **Factor 2 (Mandatory):** 
    - **Initial:** TOTP (Time-based One-Time Password) via authenticator apps.
    - **Enhanced:** WebAuthn support for hardware security keys (YubiKeys) and biometric Passkeys.
- **Session Management:** Secure, HTTP-only, SameSite=Lax cookies with short TTLs and rolling renewal.

### 1.2 Admin Schema
A new `admins` table will track credentials and MFA status:
```sql
CREATE TABLE admins (
    id BIGSERIAL PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    mfa_secret TEXT,
    mfa_enabled BOOLEAN DEFAULT FALSE,
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

---

## 2. HTMX-Driven Administration

The entire admin UI will utilize **HTMX** for high-performance partial updates, ensuring a modern user experience without the overhead of a heavy JS framework.

### 2.1 Dashboard Features
- **Real-time Search:** Filter entities as you type.
- **Inline Editing:** Quick-save changes to minor fields directly from list views.
- **Modal Workflows:** Complex operations (like linking relics) handled via htmx-loaded modals.

### 2.2 Content Lifecycle & Integrity
- **Draft/Published State:** All entities support a `status` field to allow for "work-in-progress" research.
- **Slug Management:** 
    - Auto-generated from names.
    - Real-time collision detection (e.g., if `st-nicholas` exists, suggest `st-nicholas-chicago`).

### 2.3 Activity Logging
Administrative actions (Create, Update, Delete) are logged using Go's `slog` library. This ensures a transparent record of changes in the application logs without the complexity of a database table.

---

## 3. Church Mapping & Geolocation

The church creation form will feature an integrated mapping tool:
1. **Geocoding:** Enter an address; the system uses a geocoding API to suggest coordinates.
2. **Interactive Map:** A map view (OpenLayers) shows the suggested location.
3. **Draggable Pin:** Admins can manually drag the pin to the exact church entrance.
4. **Coordinate Sync:** The pin's position automatically updates the `latitude` and `longitude` form fields via HTMX/JS integration.

---

## 4. Image Pipeline (Tigris + WebP)

Automated processing ensures that all images are performant and consistent.
- **Storage:** Tigris S3-compatible bucket.
- **Path Logic:** `uploads/{entity_type}/{id}/{version}/{filename}`.
- **Pipeline:**
    1. **Original:** Stored exactly as uploaded.
    2. **Transformation:** Using `ImageMagick` (or a Go-native wrapper like `bimg`):
        - `auto-orient` pixels based on EXIF orientation.
        - `resize` to 800px max width.
        - `strip` all metadata.
        - `convert` to WebP at 80% quality.
    3. **Database:** Links to both versions stored in the `images` table.

---

## 5. Testing & Validation Strategy

High test coverage is a mandatory requirement for the admin interface.

### 5.1 Unit Tests
- Password hashing and MFA token validation logic.
- Slug generation and collision logic.

### 5.2 Integration Tests
- Full CRUD cycles for Churches, Saints, and Relics.
- Image processing pipeline (verifying file creation and format conversion).
- Permission checks (ensuring non-MFA sessions are blocked).

### 5.3 End-to-End (E2E) Tests
- Browser-based testing for the mapping UI (pin dragging).
- Form submission and validation feedback via HTMX.

---

## 6. Technical Stack
- **Languages:** Go (Backend), HTML/CSS (Frontend).
- **Libraries:**
    - `htmx.org`: Frontend interactivity.
    - `pquerna/otp`: TOTP management.
    - `aws-sdk-go-v2`: Tigris/S3 integration.
    - `go-webauthn/webauthn`: Passkey support.
- **External Dependencies:** `ImageMagick` (magick binary).

---

## 7. Implementation Roadmap (TODOs)

### Phase 1: Authentication & Security Foundation
- [x] Create migration for \`admins\` table.
- [x] Implement \`Admin\` repository and database queries.
- [x] Create the login page and session handling logic (secure cookies).
- [x] Implement TOTP generation and verification logic.
- [x] Create the MFA enrollment flow (QR code display and verification).
- [x] Implement an \`AdminAuthMiddleware\` to protect admin routes.
- [x] **Tests:** Unit tests for hashing, TOTP, and middleware; Integration tests for login flow.

### Phase 2: Core Admin Shell & Dashboard
- [x] Design the base Admin layout (Sidebar, Main Content Area, Toasts).
- [x] Create a "Dashboard" home page with basic stats (count of churches, saints, etc.).
- [x] Set up the HTMX modal/dialog system for quick actions.
- [x] Implement Activity Logging via \`slog\`.

### Phase 3: Entity Management (CRUD)
- [x] **Saints Management:** List, Create, Edit, Delete (HTMX-powered).
- [x] **Church Management:** List, Create, Edit, Delete.
- [x] **Relic Management:** Interface to link Saints to Churches.
- [x] **Admin Management:** Invite-only system to create new administrators from the dashboard.
- [x] Implement Draft/Published status toggle for all entities.
- [x] Implement auto-slugging with collision detection.
- [x] **Tests:** CRUD integration tests for all entities; Slug collision tests.

### Phase 4: Mapping & Geolocation UI
- [x] Integrate Leaflet/OpenLayers into the Church Edit form.
- [x] Implement geocoding search box that updates the map view.
- [x] Add Draggable Pin functionality.
- [x] Ensure pin movement updates the hidden Lat/Long inputs via HTMX/JS.
- [x] **Tests:** E2E tests for address search and pin dragging.

### Phase 5: Image Pipeline & Tigris Integration
- [ ] Set up Tigris S3 client and configuration.
- [ ] Implement the multi-file upload handler.
- [ ] Integrate ImageMagick (\`magick\`) for WebP conversion and optimization.
- [ ] Implement the image management gallery (set primary, delete, edit alt text).
- [ ] **Tests:** Pipeline integration tests verifying WebP output and Tigris storage.

### Phase 6: Polish & Advanced Security
- [ ] Implement Bulk Actions (batch publish/delete).
- [ ] Add WebAuthn support (YubiKeys/Passkeys).
- [ ] Final UI/UX pass (animations, accessibility, mobile responsiveness).
- [ ] Perform a final security audit on all administrative endpoints.
