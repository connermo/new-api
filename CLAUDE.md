# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Development Commands
- **Build frontend**: `cd web && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat ../VERSION) bun run build`
- **Start backend**: `go run main.go`
- **Full development setup**: `make all` (builds frontend and starts backend)
- **Frontend dev server**: `cd web && bun run dev`
- **Frontend linting**: `cd web && bun run lint` (check) or `bun run lint:fix` (fix)

### Production Commands
- **Docker build**: Use provided `Dockerfile` and `docker-compose.yml`
- **Binary build**: `go build -o new-api main.go`

### Testing
- No explicit test commands found in package.json or makefile
- Check with maintainers for testing procedures

## Architecture Overview

This is **New API**, a large language model gateway and AI asset management system forked from One API. It's a Go backend with React frontend that provides unified API access to multiple AI providers.

### Core Architecture
- **Backend**: Go with Gin web framework
- **Frontend**: React + Vite + Semi Design UI components
- **Database**: SQLite (default), MySQL, PostgreSQL support via GORM
- **Cache**: Redis support with memory cache fallback
- **Session**: Cookie-based sessions

### Key Components

#### Backend Structure
- **`main.go`**: Application entry point, initializes all components
- **`router/`**: HTTP routing (API, dashboard, relay, video, web routes)
- **`controller/`**: HTTP handlers for different functionalities
- **`relay/`**: Core relay system with adapters for different AI providers
- **`model/`**: Database models and data access layer
- **`middleware/`**: HTTP middleware (auth, rate limiting, logging, etc.)
- **`service/`**: Business logic services
- **`common/`**: Shared utilities, constants, and configuration
- **`constant/`**: System constants and enums

#### Frontend Structure
- **`web/src/`**: React application
- **`components/`**: Reusable UI components
- **`pages/`**: Route-specific page components
- **`context/`**: React context providers (User, Theme, Status)
- **`helpers/`**: Utility functions and API helpers

#### Channel System
The relay system supports multiple AI providers through an adapter pattern:
- Each provider has its own adapter in `relay/channel/[provider]/`
- Adapters implement common interfaces for different API types
- Supported providers: OpenAI, Claude, Gemini, Baidu, Alibaba, AWS Bedrock, etc.

### Key Features
- Multi-provider AI API gateway
- User and token management
- Channel (provider) management with load balancing
- Rate limiting and quota tracking
- Real-time usage monitoring
- Payment integration (Stripe, E-pay)
- Multi-language support (i18n)
- WebSocket support for real-time APIs

### Environment Variables
Key environment variables that affect development:
- `SQL_DSN`: Database connection string
- `REDIS_CONN_STRING`: Redis connection for caching
- `GIN_MODE`: Set to "debug" for development
- `PORT`: Server port (default: 3000)
- `SESSION_SECRET`: Required for multi-instance deployments
- `CRYPTO_SECRET`: Required for encrypted Redis content

### Database Schema
- Uses GORM for ORM with automatic migrations
- Main tables: users, tokens, channels, logs, options, pricing
- Supports both single DB and separate log DB configurations

### API Structure
- **`/api/`**: Management APIs (users, channels, tokens, etc.)
- **`/v1/`**: Relay APIs that proxy to AI providers
- **`/dashboard/`**: Dashboard-specific APIs
- **`/video/`**: Video processing APIs
- WebSocket endpoints for real-time communication

### Deployment Notes
- Supports Docker deployment with volume mounting for `/data`
- Can run in distributed mode with shared Redis
- Frontend can be served separately via `FRONTEND_BASE_URL`
- Requires proper session and crypto secrets for multi-instance setups