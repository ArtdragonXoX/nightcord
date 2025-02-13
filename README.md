# nightcord-server
```
nightcord-server
├─ config.yaml
├─ go.mod
├─ go.sum
├─ internal
│  ├─ bootstrap
│  │  ├─ bootstrap.go
│  │  ├─ conf.go
│  │  ├─ executor.go
│  │  ├─ language.go
│  │  └─ server.go
│  ├─ conf
│  │  ├─ config.go
│  │  ├─ executor.go
│  │  ├─ server.go
│  │  └─ var.go
│  ├─ model
│  │  ├─ executor.go
│  │  └─ language.go
│  └─ service
│     ├─ executor
│     │  ├─ executor.go
│     │  ├─ Submit.go
│     │  └─ var.go
│     └─ language
│        ├─ language.go
│        └─ var.go
├─ main.go
├─ README.md
├─ server
│  ├─ handler
│  │  ├─ executor.go
│  │  └─ language.go
│  ├─ middlewares
│  │  └─ auth.go
│  ├─ router.go
│  ├─ routes
│  │  ├─ executor.go
│  │  └─ language.go
│  └─ server.go
└─ utils
   ├─ misc.go
   └─ yaml.go

```