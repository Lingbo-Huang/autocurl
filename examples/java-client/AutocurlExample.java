import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

public final class AutocurlExample {
    private static final String DEFAULT_BODY = """
            {
              "user_id": 10086,
              "shop_id": "shop-autocurl-demo",
              "items": [
                {
                  "sku": "sku-red-001",
                  "name": "Mechanical keyboard",
                  "quantity": 1,
                  "price_cent": 69900
                },
                {
                  "sku": "sku-blue-002",
                  "name": "USB-C cable",
                  "quantity": 3,
                  "price_cent": 3900
                }
              ],
              "address": {
                "province": "Shanghai",
                "city": "Shanghai",
                "detail": "Autocurl demo address"
              },
              "metadata": {
                "source": "intellij-idea",
                "debug_mode": "true",
                "note": "This nested body should appear in the generated cURL"
              }
            }
            """;

    private AutocurlExample() {}

    public static void main(String[] args) throws Exception {
        String target = args.length > 0 ? args[0] : "https://example.com/";
        String body = args.length > 1 ? args[1] : DEFAULT_BODY;

        HttpClient client = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(5))
                .version(HttpClient.Version.HTTP_2)
                .build();

        String querySeparator = target.contains("?") ? "&" : "?";
        HttpRequest get = requestBuilder(
                target + querySeparator + "user_id=10086&scene=intellij-plugin-demo"
        )
                .GET()
                .build();
        send(client, get);

        HttpRequest post = requestBuilder(target)
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();
        send(client, post);

        System.out.println(
                "Done. Select the POST request in Autocurl to inspect and copy its nested JSON body."
        );
    }

    private static HttpRequest.Builder requestBuilder(String target) {
        return HttpRequest.newBuilder(URI.create(target))
                .timeout(Duration.ofSeconds(15))
                .header("User-Agent", "autocurl-java-example/1")
                .header("get-info", "true")
                .header("X-Demo-Trace-ID", "autocurl-java-" + System.currentTimeMillis())
                .header("Authorization", "Bearer demo-token-not-a-real-secret")
                .header("X-API-Key", "demo-api-key");
    }

    private static void send(HttpClient client, HttpRequest request) throws Exception {
        HttpResponse<String> response = client.send(
                request,
                HttpResponse.BodyHandlers.ofString()
        );

        System.out.printf(
                "%s status=%d protocol=%s bytes=%d%n",
                request.method(),
                response.statusCode(),
                response.version(),
                response.body().length()
        );
    }
}
