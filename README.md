# 🧪 Helm Chart Validator

A web-based tool to validate Helm charts and store chart metadata in a MySQL database. Upload `.tgz` Helm charts, validate them with `helm template`, and submit metadata—all from your browser.

---

## 🚀 Features

- Upload and validate `.tgz || .tar` Helm charts
- Real-time display of `helm template` output
- Submit only if validation is successful
- Backend in Go, UI with Bootstrap & JavaScript
- MySQL database integration

---

## 🐳 How to Use

### 1. Start the application

```bash
docker compose up -d
```

### 2. CleanUp 

```
docker compose down
```
