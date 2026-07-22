# Nika Cli
Nika is a modern backend framework for Go, designed for scalability, clean architecture, and developer productivity.
 


## Commands

- [docs](./docs/README.md) - Nika CLI Documentation

Send a prompt to a local Ollama model:

```bash
nika ollema llama3.2 "Explain dependency injection in Go"
```

To generate a module from a natural-language prompt, mention creating or
building a module:

```bash
nika ollema kimi-k2.7-code:cloud "لطفن ماژول خبر واسم بساز و فیلد تصویر و عنوان و متن و تگ ها رو داشته باشه"
```


---

## Getting Started

```bash
go install github.com/nika-framework/nika-cli@latest
go build -o nika    
```
