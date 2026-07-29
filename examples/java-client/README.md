# Autocurl Java HTTP/2 example

This dependency-free Java 17 example sends two outbound requests to
`https://example.com/`:

- an HTTP/2 GET with query parameters;
- a POST with a nested JSON body and demo headers.

## Verify the JetBrains plugin in IntelliJ IDEA

1. Open this `java-client` directory as a project and let IDEA import
   `pom.xml`.
2. Open `AutocurlExample.java` and run `main` once normally to create an
   **Application** Run Configuration.
3. Select that configuration, open the **Autocurl** tool window, and click
   **Run Selected**.
4. Select the POST row. The right pane should contain a complete cURL with
   `--data-binary` followed by the nested JSON body. The GET row demonstrates
   `--http2`.
5. Click **Copy cURL**. The complete command, including the body, is now in the
   system clipboard.

If you select the GET row, seeing only the URL and headers is expected: this
example's GET request has no body. Select the POST row to verify body capture.

The console should finish with output similar to:

```text
GET status=200 protocol=HTTP_2 bytes=559
POST status=405 protocol=HTTP_2 bytes=558
Done. Select the POST request in Autocurl to inspect and copy its nested JSON body.
```

The exact byte count can change. `405 Method Not Allowed` is expected because
example.com is a documentation endpoint; the request is still captured before
the response and proves that the complete body is available.

## Run without an IDE

Java source-file mode needs no build step:

```bash
java AutocurlExample.java
```

To verify the CLI capture path:

```bash
autocurl run --all -- java AutocurlExample.java
```
