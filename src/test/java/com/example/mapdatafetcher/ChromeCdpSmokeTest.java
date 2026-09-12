package com.example.mapdatafetcher;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.logging.Level;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.openqa.selenium.TimeoutException;
import org.openqa.selenium.WebDriverException;
import org.openqa.selenium.chrome.ChromeDriver;
import org.openqa.selenium.chrome.ChromeOptions;
import org.openqa.selenium.logging.LogEntry;
import org.openqa.selenium.logging.LogType;
import org.openqa.selenium.logging.LoggingPreferences;

@Tag("cdp-smoke")
class ChromeCdpSmokeTest {

  private static final String TEST_URL = "https://example.com/";
  private static final Duration TIMEOUT = Duration.ofSeconds(15);

  private final ObjectMapper objectMapper = new ObjectMapper();

  @Test
  void capturesNetworkResponseBodyThroughCdp() throws Exception {
    ChromeOptions options = new ChromeOptions();
    options.addArguments(
        "--headless=new", "--no-sandbox", "--disable-dev-shm-usage", "--window-size=1280,720");

    LoggingPreferences loggingPreferences = new LoggingPreferences();
    loggingPreferences.enable(LogType.PERFORMANCE, Level.ALL);
    options.setCapability("goog:loggingPrefs", loggingPreferences);

    ChromeDriver driver = new ChromeDriver(options);
    try {
      Map<String, Object> enableResult = driver.executeCdpCommand("Network.enable", Map.of());
      assertNotNull(enableResult, "Network.enable should return a result");

      driver.get(TEST_URL);
      CapturedResponse response = waitForExampleResponse(driver);

      assertEquals(200, response.status(), "example.com should return HTTP 200");
      assertFalse(response.body().isBlank(), "Network.getResponseBody should return content");
    } finally {
      driver.quit();
    }
  }

  private CapturedResponse waitForExampleResponse(ChromeDriver driver) throws Exception {
    Instant deadline = Instant.now().plus(TIMEOUT);
    while (Instant.now().isBefore(deadline)) {
      List<LogEntry> entries = driver.manage().logs().get(LogType.PERFORMANCE).getAll();
      for (LogEntry entry : entries) {
        JsonNode message = objectMapper.readTree(entry.getMessage()).path("message");
        if (!"Network.responseReceived".equals(message.path("method").asText())) {
          continue;
        }

        JsonNode params = message.path("params");
        JsonNode response = params.path("response");
        if (!TEST_URL.equals(response.path("url").asText())) {
          continue;
        }

        try {
          Map<String, Object> bodyResult =
              driver.executeCdpCommand(
                  "Network.getResponseBody",
                  Map.of("requestId", params.path("requestId").asText()));
          Object body = bodyResult.get("body");
          if (body instanceof String bodyText) {
            return new CapturedResponse(response.path("status").asInt(), bodyText);
          }
        } catch (WebDriverException ignored) {
          // The response event can arrive just before its body becomes available.
        }
      }

      Thread.sleep(100L);
    }

    throw new TimeoutException("No retrievable Network.responseReceived body for " + TEST_URL);
  }

  private record CapturedResponse(int status, String body) {}
}
