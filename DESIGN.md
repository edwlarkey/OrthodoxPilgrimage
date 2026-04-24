---
version: alpha
name: Orthodox Pilgrimage Modern Plum
description: A modern, high-tech design system utilizing glassmorphism and plum accents for Orthodox Christian pilgrimage sites.
colors:
  primary: "#2c3e50"
  accent: "#530c38"
  glass-bg: "rgba(255, 255, 255, 0.85)"
  glass-border: "rgba(255, 255, 255, 0.4)"
  shadow: "rgba(83, 12, 56, 0.15)"
typography:
  brand:
    fontFamily: "Antic Didone, serif"
    fontSize: 25px
    fontWeight: 400
    lineHeight: 1
  headline:
    fontFamily: "Inter, sans-serif"
    fontSize: 19px
    fontWeight: 800
    lineHeight: 1.2
  body:
    fontFamily: "Inter, sans-serif"
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontFamily: "Inter, sans-serif"
    fontSize: 12px
    fontWeight: 700
    lineHeight: 1
    letterSpacing: 0.05em
rounded:
  sm: 6px
  md: 12px
  lg: 20px
spacing:
  xs: 4px
  sm: 8px
  md: 15px
  lg: 20px
  xl: 40px
---

# DESIGN.md

## Overview

The Orthodox Pilgrimage Modern Plum design system utilizes "glassmorphism" to create a light, airy, and high-tech interface. It uses translucency and blur effects to provide depth while maintaining a clean, modern aesthetic. The palette is professional and calm, using deep plum as its primary interactive driver.

## Colors

The palette is rooted in sophisticated slates and a vibrant plum accent.

- **Primary (#2c3e50):** A deep charcoal used for text and core structural elements.
- **Accent (#530c38):** A rich plum used for all primary interactions, links, and markers.
- **Glass BG (rgba(255, 255, 255, 0.85)):** A translucent white that forms the base of all UI panels, allowing the map background to subtly show through.

## Typography

The system uses a combination of **Antic Didone** for branding and **Inter** for all functional interface elements.

- **Brand:** Set in Antic Didone (Regular). Provides a classical, liturgical feel to the main title.
- **Headlines:** Set in Inter (Extra Bold). Compact and high-impact.
- **Body:** Inter (Regular) at 14px. Clean and optimized for digital readability.
- **Labels:** Inter (Bold) with slight letter spacing for metadata and navigation.

## Layout

The layout uses absolute positioning and floating panels ("glass cards") over a full-screen map. Panels use `backdrop-filter: blur(8px)` to maintain legibility against the complex map background. On mobile devices, panels account for system UI overlays (like the Safari navigation bar) using safe-area insets and additional bottom padding.

## Elevation & Depth

Depth is conveyed through **translucency** and **soft shadows**. Shadows use a hint of the plum accent color to create a cohesive glowing effect for elevated panels.

## Shapes

Containers and buttons use generous rounded corners (12px - 20px) to evoke a friendly, modern, and accessible interface. Small components like tags and interactive labels use `overflow-wrap` to prevent layout breaking from long strings or URLs.

## Components

### Logo & Branding
The site logo is featured prominently in the header with a height of 44px, paired with the site title in Antic Didone. The title uses a tight line-height and stacked layout on smaller screens.

### Buttons
Primary actions use solid Plum backgrounds with white text and rounded "pill" or semi-pill shapes. All interactive elements inherit the system font stack (`Inter`) for consistent weight and clarity.

### Panels
All panels (Header, Sidebar, Search) utilize the glass background and border tokens with a blur effect.

## Do's and Don'ts

- **Do** use backdrop filters for all floating UI elements.
- **Do** maintain a consistent 20px margin from the screen edges on desktop.
- **Do** use `env(safe-area-inset-bottom)` to ensure content isn't obscured by mobile system toolbars.
- **Do** allow long URLs and names to wrap or break to prevent horizontal overflow.
- **Don't** use solid, opaque backgrounds for large panels.
- **Don't** use sharp corners; prefer the defined `md` and `lg` rounding tokens.
