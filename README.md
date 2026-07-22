# Nika Cli
Nika is a modern backend framework for Go, designed for scalability, clean architecture, and developer productivity.
 


## Commands

- [docs](./docs/README.md) - Nika CLI Documentation

Configure a local Ollama agent once per project:

```bash
nika agent init ollama
nika agent "Explain dependency injection in Go"
```

The configured agent can generate modules and add routes:

```bash
nika agent "لطفن ماژول خبر واسم بساز و فیلد تصویر و عنوان و متن و تگ ها رو داشته باشه"
nika agent "یک روت ایجاد دیتای ماک روی ماژول news اضافه کن"
```


---

## Getting Started

```bash
go install github.com/nika-framework/nika-cli@latest
go build -o nika    
```
