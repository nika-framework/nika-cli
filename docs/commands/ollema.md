# nika ollema (legacy)

> Superseded by [`nika agent`](../cli/agent.md). `nika ollema` is a single-shot
> Ollama command with no tool loop: it either prints a reply, generates a
> module from a keyword-matched prompt, or adds a mock route. Prefer
> `nika agent`, which can do all of that and anything else, on any provider.

The command was defined but never registered in earlier versions, so
`nika ollema` reported "unknown command". It works now.

## Usage

```bash
nika ollema <model> <text>
```

For text containing spaces, wrap the prompt in quotes:

```bash
nika ollema llama3.2 "Explain dependency injection in Go"
```

To generate a module from a natural-language request:

```bash
nika ollema kimi-k2.7-code:cloud "لطفن ماژول خبر واسم بساز از منگو استفاده کن و فیلد تصویر و عنوان و متن و تگ ها رو داشته باشه"
```

The model returns a structured definition containing an English module name,
field names, Go types, and required flags. The CLI validates that definition,
then runs the normal resource generator without interactive questions.

## Mock routes

```bash
nika ollema kimi-k2.7-code:cloud "یک روت ایجاد دیتای ماک روی ماژول news اضافه کن"
```

The CLI locates the module, its model, and its controllers. In a microservice
workspace it finds which service owns the module and asks only when the same
module name exists in more than one. If the module has multiple controllers, it
asks which should receive the route. It then creates
`<src>/news/controllers/mock.go` and registers the route on that controller.
The default route is `POST /newss/mock`, using the module's existing `Create`
service method.

The generated handler separates the field from the method:

```go
CreateMock func(*gin.Context) `route:"POST:/newss/mock"`

// in the constructor
c.CreateMock = c.CreateMockHandler
```

`CreateMockHandler` includes Swagger annotations. Model support here is limited
to MongoDB; use `nika agent` for SQL-backed modules.

## Endpoint

The command uses `http://localhost:11434` by default. Set `OLLAMA_HOST` when
Ollama is listening on another local address:

```bash
OLLAMA_HOST=http://localhost:11434 nika ollema llama3.2 "Hello"
```

## Equivalent with `nika agent`

```bash
nika agent init ollama
nika agent "Create a news module with title, text, image, and tags"
nika agent "add a mock data creation route to the news module"
```
