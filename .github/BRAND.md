# Brand Guidelines

![Project Logo](assets/LOGO.png)

> *"I am because we are"* - Ubuntu Philosophy

Welcome to the brand guidelines for Kartoza Timesheet App. These guidelines ensure consistent visual identity and design philosophy across all project materials.

## Design Philosophy

Our brand embodies the Ubuntu philosophy - "I am because we are" - emphasizing collaboration, community, and shared growth. The design reflects:

- **Inclusivity:** Welcoming to all contributors regardless of background
- **Community:** Strong emphasis on collaboration and shared success  
- **Quality:** Professional presentation with attention to detail
- **Accessibility:** Design that works for everyone
- **Authenticity:** African-inspired elements honoring Ubuntu origins

## Color Palette

### Primary Colors

| Color | Hex Code | Usage |
|-------|----------|--------|
| Ubuntu Orange | `#E95420` | Primary brand color, CTAs, highlights |
| Warm Brown | `#8B4513` | Secondary accent, earth tones |
| Deep Charcoal | `#2C3E50` | Text, headers, professional elements |
| Warm White | `#FDF6E3` | Backgrounds, negative space |

### Accent Colors

| Color | Hex Code | Usage |
|-------|----------|--------|
| Sunset Orange | `#FF6B35` | Interactive elements, warnings |
| Earth Green | `#7CB342` | Success states, positive actions |
| Sky Blue | `#42A5F5` | Information, links, navigation |
| Rich Purple | `#8E24AA` | Special features, premium elements |

### Neutral Palette

| Color | Hex Code | Usage |
|-------|----------|--------|
| Light Grey | `#F5F5F5` | Subtle backgrounds, dividers |
| Medium Grey | `#BDBDBD` | Borders, inactive elements |
| Dark Grey | `#616161` | Secondary text, icons |

## Typography

### Primary Font: Ubuntu

```css
font-family: 'Ubuntu', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
```

- **Headers:** Ubuntu Bold
- **Body Text:** Ubuntu Regular  
- **Code:** Ubuntu Mono
- **UI Elements:** Ubuntu Medium

### African-Inspired Alternatives

For special occasions and cultural elements:

- **Display Headers:** Amandla (if available)
- **Cultural Elements:** African Sans or similar
- **Fallbacks:** 'Segoe UI', Roboto, Arial

### Typography Scale

| Element | Size | Weight | Line Height |
|---------|------|---------|-------------|
| H1 | 3.5rem | 700 | 1.2 |
| H2 | 2.5rem | 600 | 1.3 |
| H3 | 2rem | 600 | 1.3 |
| H4 | 1.5rem | 500 | 1.4 |
| H5 | 1.25rem | 500 | 1.4 |
| Body Large | 1.125rem | 400 | 1.6 |
| Body Regular | 1rem | 400 | 1.6 |
| Body Small | 0.875rem | 400 | 1.5 |
| Caption | 0.75rem | 400 | 1.4 |

## Logo Usage

### Primary Logo

The primary logo combines the Ubuntu-inspired symbol with clean, professional typography. It should be used:

- In headers and navigation
- On documentation covers
- In presentations and marketing materials
- As favicon (simplified version)

### Logo Variations

1. **Full Logo:** Symbol + wordmark (preferred)
2. **Symbol Only:** For small spaces and favicons
3. **Wordmark Only:** For very wide layouts
4. **Monochrome:** Black/white versions for print

### Clear Space

Maintain clear space around the logo equal to the height of the Ubuntu symbol.

### Minimum Sizes

- **Digital:** 120px wide minimum
- **Print:** 1 inch wide minimum
- **Favicon:** 32px x 32px

### Don'ts

- Don't stretch or distort the logo
- Don't use unauthorized colors
- Don't place on busy backgrounds
- Don't use outdated versions

## Mascot Character

*[To be developed based on logo design]*

The mascot embodies:
- Friendly and approachable personality
- Ubuntu philosophy of community
- Professional yet warm demeanor
- Inclusive representation

## Visual Elements

### Icons

- **Style:** Line icons with 2px stroke weight
- **Corners:** Rounded (4px radius)
- **Size:** 24px standard, scalable
- **Color:** Inherit from context or brand colors

### Buttons

#### Primary Buttons
```css
background: #E95420;
color: #FFFFFF;
border-radius: 8px;
padding: 12px 24px;
font-weight: 500;
```

#### Secondary Buttons
```css
background: transparent;
color: #E95420;
border: 2px solid #E95420;
border-radius: 8px;
padding: 10px 22px;
font-weight: 500;
```

### Cards and Containers

```css
background: #FFFFFF;
border-radius: 12px;
box-shadow: 0 2px 8px rgba(0,0,0,0.1);
border: 1px solid #F5F5F5;
```

### Forms

```css
input, textarea {
  border: 2px solid #BDBDBD;
  border-radius: 8px;
  padding: 12px 16px;
  font-family: 'Ubuntu', sans-serif;
}

input:focus {
  border-color: #E95420;
  outline: none;
  box-shadow: 0 0 0 3px rgba(233, 84, 32, 0.1);
}
```

## Application in Different Media

### Web Interface

- Use full color palette
- Implement responsive design
- Ensure accessibility contrast ratios
- Follow material design principles

### Documentation

- Consistent header styling with logo
- Ubuntu philosophy quotes where appropriate
- Clean typography hierarchy
- Generous white space

### Marketing Materials

- Bold use of Ubuntu Orange
- Professional photography
- Community-focused messaging
- Clear call-to-actions

### Social Media

- Consistent profile images
- Brand color overlays on photos
- Ubuntu philosophy hashtags
- Community celebration posts

## Accessibility Guidelines

### Color Contrast

All text must meet WCAG 2.1 AA standards:
- Normal text: 4.5:1 minimum
- Large text: 3:1 minimum
- UI elements: 3:1 minimum

### Alternative Text

All images must include descriptive alt text, especially:
- Logo: "Kartoza Timesheet App logo"
- Decorative images: Describe content and context
- UI icons: Describe function

### Typography

- Minimum 16px font size for body text
- Clear hierarchy with proper heading structure
- Sufficient line height for readability

## Implementation Notes

### CSS Custom Properties

```css
:root {
  --color-ubuntu-orange: #E95420;
  --color-warm-brown: #8B4513;
  --color-deep-charcoal: #2C3E50;
  --color-warm-white: #FDF6E3;
  
  --font-family-primary: 'Ubuntu', -apple-system, BlinkMacSystemFont, sans-serif;
  --font-family-code: 'Ubuntu Mono', 'Fira Code', monospace;
  
  --border-radius-small: 4px;
  --border-radius-medium: 8px;
  --border-radius-large: 12px;
  
  --spacing-xs: 4px;
  --spacing-sm: 8px;
  --spacing-md: 16px;
  --spacing-lg: 24px;
  --spacing-xl: 32px;
}
```

### Asset Generation

When generating new assets, use these prompts:

> Create a professional, modern design that embodies the Ubuntu philosophy of "I am because we are". Use Ubuntu Orange (#E95420) as the primary color, incorporate African-inspired elements subtly, and ensure the design feels welcoming, inclusive, and community-focused. The style should be clean, accessible, and suitable for a professional timesheet application.

## Brand Voice and Messaging

### Tone of Voice

- **Friendly but Professional:** Approachable yet competent
- **Inclusive:** Welcoming to all users and contributors  
- **Collaborative:** Emphasizing teamwork and community
- **Helpful:** Always trying to solve problems and add value
- **Authentic:** True to Ubuntu philosophy and values

### Key Messages

1. **Community First:** "Built by the community, for the community"
2. **Ubuntu Philosophy:** "I am because we are"
3. **Quality Focused:** "Excellence through collaboration"
4. **Open Source:** "Transparency, accessibility, shared growth"
5. **Professional:** "Enterprise-ready, community-driven"

### Writing Guidelines

- Use inclusive language
- Avoid jargon when possible
- Be direct but warm
- Emphasize benefits and outcomes
- Include calls-to-action that invite participation

---

*These guidelines ensure consistent brand presentation while honoring the Ubuntu philosophy that drives our community. For questions or clarifications, contact tim@kartoza.com.*