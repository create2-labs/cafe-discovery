# Cafe Discovery Frontend

Modern Vue.js 3 frontend for the Cafe Discovery quantum security scanner.

## Features

- 🔐 Authentication (Sign In / Sign Up)
- 📊 Dashboard with security statistics
- 🔍 Wallet scanning functionality
- 📋 Security-focused scan list view (Snyk/Trivy style)
- 🎨 Modern, responsive UI with Tailwind CSS
- 🔒 JWT-based authentication

## Tech Stack

- **Vue 3** - Progressive JavaScript framework
- **Vite** - Next generation frontend tooling
- **Vue Router** - Official router for Vue.js
- **Pinia** - State management
- **Tailwind CSS** - Utility-first CSS framework
- **Axios** - HTTP client

## Getting Started

### Install Dependencies

```bash
npm install
```

### Development

```bash
npm run dev
```

The app will be available at `http://localhost:3000`

### Build for Production

```bash
npm run build
```

### Preview Production Build

```bash
npm run preview
```

## Environment Variables

Create a `.env` file in the frontend directory:

```env
VITE_API_URL=http://localhost:8080
```

## Project Structure

```
frontend/
├── src/
│   ├── components/     # Reusable Vue components
│   ├── views/          # Page components
│   ├── services/       # API services
│   ├── stores/         # Pinia stores
│   ├── router/         # Vue Router configuration
│   └── style.css       # Global styles
├── index.html
├── package.json
└── vite.config.js
```

## API Integration

The frontend communicates with the backend API at `http://localhost:8080` by default. Make sure the backend server is running before starting the frontend.

## Features Overview

### Authentication
- Sign up with email and password
- Sign in with existing credentials
- JWT token-based authentication
- Automatic token refresh

### Dashboard
- Overview statistics (Total Scans, High Risk, Medium Risk, Safe)
- Recent scans list
- Quick scan functionality

### Scans View
- Security-focused list view
- Filter by risk level and type
- Search functionality
- Detailed scan information
- Security recommendations

### Scan Wallet
- Real-time wallet scanning
- Multi-network support
- Risk assessment
- Security recommendations

