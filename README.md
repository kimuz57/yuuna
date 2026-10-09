# Yuuna

แชทบอท AI (Google Gemini) แบบ Full-Stack — ผู้ใช้สนทนาได้ทั้งแบบผู้เยี่ยมชม (Guest) และแบบล็อกอินด้วย Google เพื่อเก็บประวัติสนทนาและจัดการห้องแชท (Sessions)

## สถาปัตยกรรม

```
┌─────────────────────────────┐        ┌──────────────────────────────┐
│  Frontend (yuuna)           │        │  Backend (api-yuuna)         │
│  React 19 + Vite 8          │  /api  │  Go + Fiber v2               │
│  http://localhost:5173      │ ─────► │  127.0.0.1:8080 (เฉพาะ       │
│                             │ proxy  │  localhost เท่านั้น)         │
│  เรียก API ด้วย Relative    │        │                              │
│  Path (/api/...)            │        │  ├─ PostgreSQL (GORM)        │
│  + credentials: "include"   │        │  ├─ Redis (quota/cache)      │
└─────────────────────────────┘        │  └─ Google ID Token + JWT    │
                                       └──────────────────────────────┘
```

- **Backend ไม่เปิดรับการเชื่อมต่อจากภายนอก**: bind เฉพาะ `127.0.0.1` (บังคับโดย `loopbackOnly()` ห้ามตั้ง `0.0.0.0`)
- **Frontend ใช้ Relative Path ทั้งหมด**: ทุกคำขอไปที่ `/api/...` แล้วให้ Vite Dev Proxy (dev) หรือ Reverse Proxy (production) ส่งต่อไปยัง `127.0.0.1:8080`
- **CORS เจาะจงโดเมน**: อนุญาตเฉพาะ `https://yuuna.tcmg.me`, `http://localhost:3000` และ `FRONTEND_URL` ใน `.env`

## Tech Stack

| ส่วน | เทคโนโลยี |
|---|---|
| Frontend | React 19, Vite 8, Tailwind CSS 4, react-router-dom 7, `@react-oauth/google` (Google Identity Services) |
| Backend | Go, Fiber v2, GORM (PostgreSQL), go-redis, golang-jwt v5, `google.golang.org/api/idtoken` |
| AI | Google Gemini |
| Auth | Google Sign-In → ID Token → HttpOnly Session Cookie (JWT HS256, อายุ 72 ชม.) |

## โครงสร้างโปรเจกต์

```
yuuna/
├── backend/                  # Go API (ชื่อโมดูล: yuuna)
│   ├── main.go               # bootstrap, bind 127.0.0.1, CORS, trusted proxy
│   ├── config/               # โหลด .env (godotenv)
│   ├── routes/routes.go      # ลงทะเบียน endpoint ทั้งหมด
│   ├── middleware/auth.go    # Protected() — ตรวจ JWT จาก HttpOnly cookie
│   ├── controller/           # auth, chat, session, guest_chat, migrate
│   ├── model/                # User, ChatSession, ChatMessage (GORM)
│   ├── database/             # เชื่อมต่อ PostgreSQL + AutoMigrate
│   ├── store/redis.go        # Redis (quota ผู้เยี่ยมชม + cache)
│   └── core/                 # Gemini client + prompt
└── frontend/                 # React SPA
    ├── vite.config.js        # dev/preview proxy /api → 127.0.0.1:8080
    └── src/
        ├── App.jsx           # GoogleOAuthProvider + routes
        └── pages/Chat.jsx    # หน้าจอแชททั้งหมด + จัดการ auth state
```

## เริ่มต้นใช้งาน (Development)

### กำหนดการ (Prerequisites)

- Go 1.26+
- Node.js 20+
- PostgreSQL, Redis
- Google OAuth Client ID (แบบ **Web application**)

### 1. Backend

```bash
cd backend
```

สร้างไฟล์ `backend/.env` (ไฟล์นี้ gitignored — ไม่ได้มาพร้อมเมื่อ clone):

```env
PORT=8080
FRONTEND_URL=http://localhost:5173
DATABASE_URL=host=localhost user=postgres password=postgres dbname=yuna_db port=5433 sslmode=disable
GEMINI_API_KEY=your-gemini-api-key
GEMINI_MODEL=gemini-3.5-flash-lite

# จำเป็น — ไม่ตั้งค่าแล้วเซิร์ฟเวอร์จะ fail-fast ตอนบูต
JWT_SECRET=<สุ่ม 32 bytes เช่น openssl rand -hex 32>
GOOGLE_CLIENT_ID=<your-client-id>.apps.googleusercontent.com

# (ทางเลือก)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
ALLOWED_ORIGINS=https://yuuna.tcmg.me,http://localhost:3000
TRUSTED_PROXIES=127.0.0.1,::1
HOST=127.0.0.1
```

รัน:

```bash
go run .
```

### 2. Frontend

```bash
cd frontend
npm install
```

สร้างไฟล์ `frontend/.env`:

```env
VITE_GOOGLE_CLIENT_ID=<your-client-id>.apps.googleusercontent.com
```

รัน:

```bash
npm run dev        # http://localhost:5173 (proxy /api → 127.0.0.1:8080)
```

> หมายเหตุ: ตั้งค่า **Authorized JavaScript origin** ของ Google OAuth ให้รวม `http://localhost:5173` ด้วย

## ระบบ Authentication

สถานะล็อกอินถูกตัดสินใจ **บนเซิร์ฟเวอร์เท่านั้น** ผ่าน HttpOnly Session Cookie — ไม่มีการเก็บ token หรือข้อมูลผู้ใช้ไว้ใน `localStorage`

### Flow

1. **ล็อกอิน**: Frontend รับ ID Token จาก Google (`@react-oauth/google`) → `POST /api/auth/google`
2. Backend **verify ID Token กับ Google** (`idtoken.Validate` — บังคับ Audience = `GOOGLE_CLIENT_ID` ของเซิร์ฟเวอร์) → upsert ผู้ใช้ → ออก JWT (HS256, 72 ชม.) → ตั้ง cookie:
   - `HttpOnly: true` (JS อ่านไม่ได้ ป้องกัน XSS)
   - `Secure: true` (HTTPS เท่านั้น)
   - `SameSite: Lax`, `Path: /`
   - JWT ไม่ถูกส่งใน response body
3. **ตรวจสถานะ**: ทุกครั้งที่เปิดหน้าเว็บ Frontend ยิง `GET /api/auth/me` → 200 = ล็อกอินแล้ว (คืนโปรไฟล์), 401 = เคลียร์ state แล้วแสดงปุ่มล็อกอิน
4. **ปกป้อง Route**: ทุก endpoint ในกลุ่ม protected ถูกดักด้วย `middleware.Protected()` ซึ่งอ่าน cookie และ verify JWT ก่อนเสมอ
5. **ล็อกเอาต์**: `POST /api/auth/logout` → ทำลาย cookie ฝั่งเซิร์ฟเวอร์

### Endpoint

| Method | Path | Auth | คำอธิบาย |
|---|---|---|---|
| POST | `/api/auth/google` | Public | รับ ID Token จาก Google แล้วออก HttpOnly cookie |
| POST | `/api/auth/logout` | Public | ทำลาย session cookie |
| GET | `/api/auth/me` | Cookie | คืนโปรไฟล์ผู้ใช้ปัจจุบัน หรือ `401` |
| POST | `/api/guest/chat` | Public | แชทแบบผู้เยี่ยมชม (จำกัด 5 ข้อความ/IP/24 ชม. → `403 QUOTA_EXCEEDED`) |
| GET | `/api/sessions` | Cookie | รายชื่อห้องแชท |
| PUT | `/api/sessions/:id` | Cookie | เปลี่ยนชื่อห้องแชท |
| DELETE | `/api/sessions/:id` | Cookie | ลบห้องแชท |
| POST | `/api/chat` | Cookie | ส่งข้อความ (SSE stream) |
| GET | `/api/history?session_id=` | Cookie | ประวัติในห้องแชท |
| DELETE | `/api/history` | Cookie | ล้างประวัติ |
| POST | `/api/history/migrate` | Cookie | ย้ายประวัติ guest → ผู้ใช้ |

> ทุกคำขอจาก Frontend ต้องมี `credentials: "include"` เพื่อส่ง cookie ไปด้วย

## คำสั่ง Build / ตรวจสอบ

```bash
# Backend
cd backend
go build ./...
go vet ./...

# Frontend
cd frontend
npm run lint
npm run build
```

## หมายเหตุด้านความปลอดภัย

- `backend/.env` / `frontend/.env` (รวม `.env.production`) เป็น **gitignored** — เมื่อ clone ใหม่ต้องสร้างเอง โดยเฉพาะ `JWT_SECRET` และ `GOOGLE_CLIENT_ID` ที่บังคับ fail-fast ตอนบูต
- Backend bind ที่ `127.0.0.1` เท่านั้น — เข้าถึงจากเครื่องอื่นโดยตรงจะเชื่อมต่อไม่ได้ (ต้องผ่าน reverse proxy บนเครื่องเดียวกัน)
- อ่าน `X-Forwarded-For` เฉพาะเมื่อ request มาจาก proxy บน loopback เท่านั้น (`EnableTrustedProxyCheck`) ป้องกันการ Spoof หัวข้อ
- ห้ามวาง Client Secret ของ Google ไว้ในฝั่ง Frontend — ฝั่ง backend ใช้แค่ Client ID เพื่อ verify ID Token
