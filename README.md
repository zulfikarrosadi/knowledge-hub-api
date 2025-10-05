# Knowledge Hub WebSocket Backend

A lightweight Go backend designed to handle real-time synchronization between devices for the **Knowledge Hub** project. This backend uses Go’s standard library with **Gorilla WebSocket** for efficient bi-directional communication.

> 🚧 **Status:** Work in Progress — features and architecture may still change.

---

## 📘 Overview

This backend serves as the real-time layer for the Knowledge Hub app, enabling users to:

* Sync notes and updates between multiple devices instantly.
* Connect devices using a temporary OTP-based pairing system (no authentication needed).
* Handle offline-first updates with conflict resolution (planned feature).

---

## 🏗️ Tech Stack

* **Language:** Go
* **WebSocket Library:** [Gorilla WebSocket](https://github.com/gorilla/websocket)
* **HTTP:** Go `net/http` standard library
* **Storage (planned):** SQLite or in-memory cache for session & pairing data

---

## ⚙️ Features (Planned & Implemented)

| Feature                    | Status         |
| -------------------------- | -------------- |
| Basic WebSocket Connection | ✅ Implemented  |
| Broadcast Message Handling | ✅ Implemented  |
| Device Pairing with OTP    | 🧩 In Progress |
| Message Persistence        | ⏳ Planned      |
| Encryption Between Devices | ⏳ Planned      |

---

## 🚀 Getting Started

### 1. Clone Repository

```bash
git clone https://github.com/zulfikarrosadi/knowledge-hub-api.git
cd knowledge-hub-api
```

### 2. Run the Server

```bash
go run main.go
```

The server will start on `localhost:3000` by default.

### 3. Test WebSocket Connection

You can test using any WebSocket client or simple HTML page:

```javascript
const ws = new WebSocket('ws://localhost:3000/ws');
ws.onopen = () => console.log('connected');
ws.onmessage = (e) => console.log('message:', e.data);
```

---

## 📡 API Routes (Draft)

| Route   | Method | Description                             |
| ------- | ------ | --------------------------------------- |
| `/ws`   | GET    | Establish WebSocket connection          |
| `/pair` | POST   | Generate or validate device OTP pairing |
| `/sync` | POST   | (Future) Handle offline data sync       |

---

## 🧠 Project Goal

This backend is part of a broader initiative to build a **decentralized personal knowledge system** that:

* Works seamlessly across devices without accounts.
* Keeps user data local-first and encrypted.
* Enables real-time collaboration and note syncing.

---

## 🧩 Next Steps

* [ ] Implement OTP pairing logic.
* [ ] Add lightweight session store.
* [ ] Handle reconnections and disconnections gracefully.
* [ ] Add conflict resolution for offline notes.

---

## 🤝 Contributing

Contributions and suggestions are welcome! Please open an issue or submit a pull request.

---

## 📄 License

MIT License © 2025
