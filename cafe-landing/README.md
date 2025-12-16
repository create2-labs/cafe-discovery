# CAFE Landing Page

A modern, responsive landing page for CAFE (Crypto-Agility Framework for Ethereum), a quantum-resistant cryptography solution for Ethereum.

## Features

- 🎨 **Modern Design** - Beautiful gradient-based UI with dark theme
- 📱 **Fully Responsive** - Works seamlessly on desktop, tablet, and mobile devices
- ⚡ **Smooth Animations** - Interactive elements with smooth transitions and scroll effects
- 🔍 **SEO Optimized** - Semantic HTML with proper meta tags
- 🚀 **Performance** - Lightweight and fast-loading

## Sections

1. **Hero Section** - Compelling introduction with key statistics
2. **Problem Section** - Explains the quantum computing threat
3. **Solution Section** - Introduces CAFE and its approach
4. **Features Section** - Details the four integrated layers (Discovery, Agility, Remediation, Infrastructure)
5. **Technology Section** - Showcases the tech stack (ERC-4337, PQC, ZK, etc.)
6. **CTA Section** - Call-to-action for engagement
7. **Footer** - Navigation and company information

## Getting Started

### Option 1: Simple Local Server

1. Open the project directory
2. Use Python's built-in server:
   ```bash
   python3 -m http.server 8000
   ```
3. Open `http://localhost:8000` in your browser

### Option 2: Node.js Server

1. Install a simple HTTP server:
   ```bash
   npx http-server -p 8000
   ```
2. Open `http://localhost:8000` in your browser

### Option 3: Direct File Opening

Simply open `index.html` in your web browser (note: some features may be limited without a server).

## Project Structure

```
cafe-landing/
├── index.html      # Main HTML structure
├── styles.css      # All styling and responsive design
├── script.js       # Interactive features and animations
├── desc.txt        # Source content reference
└── README.md       # This file
```

## Customization

### Colors

Edit the CSS variables in `styles.css`:

```css
:root {
    --primary: #6366f1;
    --secondary: #8b5cf6;
    --accent: #ec4899;
    /* ... */
}
```

### Content

Modify the content directly in `index.html`. Each section is clearly marked with semantic HTML.

### Fonts

The page uses Inter font from Google Fonts. To change, update the font link in `index.html` and the font-family in `styles.css`.

## Browser Support

- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)

## Technologies Used

- HTML5
- CSS3 (with CSS Variables, Grid, Flexbox)
- Vanilla JavaScript (ES6+)
- Google Fonts (Inter)

## License

© 2025 CAFE. All rights reserved.

