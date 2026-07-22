# nika agent (formerly nika ollema)

The standalone `nika ollema` command has been replaced by the unified
`nika agent` command. Configure a provider and run the same workflows with:

```bash
nika agent init ollama
nika agent "یک روت ایجاد دیتای ماک روی ماژول news اضافه کن"
```

Send a prompt to a local [Ollama](https://ollama.com) model and print the response.
When the prompt asks for a module, Nika extracts its fields and generates a
complete resource using the same templates as `nika generate res`.

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

To add a mock-data route to an existing module:

```bash
nika ollema kimi-k2.7-code:cloud "یک روت ایجاد دیتای ماک روی ماژول news اضافه کن"
```

The CLI checks that the module, MongoDB model, and controllers exist. If the
module has multiple controllers, it asks which controller should receive the
route. It then creates `src/news/controllers/mock.go` and registers the route
on the selected controller. The default route is `POST /newss/mock` and it
uses the module's existing `Create` service method.

The command uses `http://localhost:11434` by default. Set `OLLAMA_HOST` when
Ollama is listening on another local address:

```bash
OLLAMA_HOST=http://localhost:11434 nika ollema llama3.2 "Hello"
```