import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

public final class AutocurlExample {
    private AutocurlExample() {}

    public static void main(String[] args) throws Exception {
        String target = args.length > 0 ? args[0] : "https://example.com/";
        HttpRequest.Builder builder = HttpRequest.newBuilder(URI.create(target))
                .timeout(Duration.ofSeconds(15))
                .header("User-Agent", "autocurl-java-example/1");

        if (args.length > 1) {
            builder.header("Content-Type", "application/json")
                    .POST(HttpRequest.BodyPublishers.ofString(args[1]));
        } else {
            builder.GET();
        }

        HttpClient client = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(5))
                .version(HttpClient.Version.HTTP_2)
                .build();
        HttpResponse<String> response = client.send(
                builder.build(),
                HttpResponse.BodyHandlers.ofString()
        );

        System.out.printf(
                "status=%d protocol=%s bytes=%d%n",
                response.statusCode(),
                response.version(),
                response.body().length()
        );
    }
}
