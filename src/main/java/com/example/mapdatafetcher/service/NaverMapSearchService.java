package com.example.mapdatafetcher.service;

import com.example.mapdatafetcher.config.NaverMapSeleniumProperties;
import com.example.mapdatafetcher.dto.NaverMapCoordinateSearchRequest;
import com.example.mapdatafetcher.dto.NaverMapSearchRequest;
import com.example.mapdatafetcher.exception.CaptureException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.Base64;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.logging.Level;
import org.openqa.selenium.By;
import org.openqa.selenium.Dimension;
import org.openqa.selenium.Keys;
import org.openqa.selenium.PageLoadStrategy;
import org.openqa.selenium.StaleElementReferenceException;
import org.openqa.selenium.TimeoutException;
import org.openqa.selenium.WebDriverException;
import org.openqa.selenium.WebElement;
import org.openqa.selenium.chrome.ChromeDriver;
import org.openqa.selenium.chrome.ChromeOptions;
import org.openqa.selenium.logging.LogEntries;
import org.openqa.selenium.logging.LogEntry;
import org.openqa.selenium.logging.LogType;
import org.openqa.selenium.logging.LoggingPreferences;
import org.openqa.selenium.support.ui.ExpectedConditions;
import org.openqa.selenium.support.ui.WebDriverWait;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.web.util.UriComponentsBuilder;
import org.springframework.web.util.UriUtils;

@Service
public class NaverMapSearchService {

  private static final Logger LOGGER = LoggerFactory.getLogger(NaverMapSearchService.class);
  private static final Duration PAGE_LOAD_TIMEOUT = Duration.ofSeconds(90);
  private static final Duration ELEMENT_WAIT_TIMEOUT = Duration.ofSeconds(30);
  private static final Duration SEARCH_RESPONSE_TIMEOUT = Duration.ofSeconds(60);
  private static final String DEFAULT_MAP_CAMERA = "15.00,0,0,0,dh";
  private static final By SEARCH_INPUT_SELECTOR =
      By.cssSelector(
          "input.input_search, input[type='search'], input[placeholder], input[aria-label]");

  private final ObjectMapper objectMapper;
  private final NaverMapSeleniumProperties properties;

  public NaverMapSearchService(ObjectMapper objectMapper, NaverMapSeleniumProperties properties) {
    this.objectMapper = objectMapper;
    this.properties = properties;
  }

  public JsonNode search(NaverMapSearchRequest request) {
    LOGGER.info("Starting Naver keyword search for page {}", request.page());
    ChromeDriver driver = null;
    String stage = "browser_startup";
    try {
      driver = createDriver();
      driver.executeCdpCommand("Network.enable", Map.of());
      int targetPage = request.page();
      stage = "page_navigation";
      driver.get(buildSearchUrl(request.q()));
      JsonNode result = captureSearchResults(driver, targetPage);
      LOGGER.info("Naver keyword search completed for page {}", targetPage);
      return result;
    } catch (CaptureException exception) {
      LOGGER.warn(
          "Naver keyword search failed: category={}, stage={}",
          exception.category(),
          exception.stage(),
          exception);
      throw exception;
    } catch (Exception exception) {
      CaptureException failure = captureFailure(stage, exception);
      LOGGER.warn(
          "Naver keyword search failed: category={}, stage={}",
          failure.category(),
          failure.stage(),
          exception);
      throw failure;
    } finally {
      quitQuietly(driver);
    }
  }

  public JsonNode searchByCoordinate(NaverMapCoordinateSearchRequest request) {
    LOGGER.info("Starting Naver coordinate search for page {}", request.page());
    ChromeDriver driver = null;
    String stage = "browser_startup";
    try {
      driver = createDriver();
      driver.executeCdpCommand("Network.enable", Map.of());
      int targetPage = request.page();
      stage = "page_navigation";
      driver.get(buildCoordinateUrl(request.longitude(), request.latitude()));
      stage = "search_input";
      submitSearchKeyword(driver, request.query());
      JsonNode result = captureSearchResults(driver, targetPage);
      LOGGER.info("Naver coordinate search completed for page {}", targetPage);
      return result;
    } catch (CaptureException exception) {
      LOGGER.warn(
          "Naver coordinate search failed: category={}, stage={}",
          exception.category(),
          exception.stage(),
          exception);
      throw exception;
    } catch (Exception exception) {
      CaptureException failure = captureFailure(stage, exception);
      LOGGER.warn(
          "Naver coordinate search failed: category={}, stage={}",
          failure.category(),
          failure.stage(),
          exception);
      throw failure;
    } finally {
      quitQuietly(driver);
    }
  }

  private CaptureException captureFailure(String stage, Exception exception) {
    Throwable cause = exception;
    while (cause != null) {
      if (cause instanceof InterruptedException) {
        Thread.currentThread().interrupt();
        return new CaptureException("cancelled", stage, "Capture was interrupted", exception);
      }
      if (cause instanceof TimeoutException) {
        return new CaptureException("timeout", stage, "Capture stage timed out", exception);
      }
      cause = cause.getCause();
    }
    return new CaptureException("capture_error", stage, "Capture stage failed", exception);
  }

  private void quitQuietly(ChromeDriver driver) {
    if (driver == null) {
      return;
    }
    try {
      driver.quit();
    } catch (WebDriverException exception) {
      LOGGER.debug("Failed to close Naver search browser cleanly", exception);
    }
  }

  private ChromeDriver createDriver() {
    ChromeOptions options = new ChromeOptions();
    options.setPageLoadStrategy(PageLoadStrategy.EAGER);
    options.addArguments(
        "--disable-gpu",
        "--no-sandbox",
        "--disable-dev-shm-usage",
        "--disable-setuid-sandbox",
        "--window-size=1367,904");

    if (properties.headless()) {
      options.addArguments("--headless=new");
    }

    LoggingPreferences loggingPreferences = new LoggingPreferences();
    loggingPreferences.enable(LogType.PERFORMANCE, Level.ALL);
    options.setCapability("goog:loggingPrefs", loggingPreferences);

    ChromeDriver driver = new ChromeDriver(options);
    driver.manage().timeouts().pageLoadTimeout(PAGE_LOAD_TIMEOUT);
    driver.manage().window().setSize(new Dimension(1367, 904));
    return driver;
  }

  private String buildSearchUrl(String query) {
    return properties.searchUrl() + UriUtils.encodePathSegment(query, StandardCharsets.UTF_8);
  }

  private String buildCoordinateUrl(Double longitude, Double latitude) {
    String searchUrl = properties.searchUrl();
    int searchPathIndex = searchUrl.indexOf("/search/");
    String baseUrl = searchPathIndex >= 0 ? searchUrl.substring(0, searchPathIndex) : searchUrl;
    return UriComponentsBuilder.fromUriString(baseUrl)
        .queryParam("lng", longitude)
        .queryParam("lat", latitude)
        .queryParam("c", DEFAULT_MAP_CAMERA)
        .build(true)
        .toUriString();
  }

  private void submitSearchKeyword(ChromeDriver driver, String query) {
    WebElement searchInput = waitForVisibleSearchInput(driver);
    clearPerformanceLogs(driver);
    searchInput.click();
    searchInput.sendKeys(Keys.chord(Keys.CONTROL, "a"), Keys.DELETE);
    searchInput.sendKeys(query);
    searchInput.sendKeys(Keys.ENTER);
  }

  private WebElement waitForVisibleSearchInput(ChromeDriver driver) {
    WebDriverWait wait = new WebDriverWait(driver, ELEMENT_WAIT_TIMEOUT);
    return wait.until(
        driverInstance ->
            driverInstance.findElements(SEARCH_INPUT_SELECTOR).stream()
                .filter(WebElement::isDisplayed)
                .findFirst()
                .orElse(null));
  }

  private JsonNode captureSearchResults(ChromeDriver driver, int targetPage) throws Exception {
    JsonNode firstPageResponse;
    try {
      firstPageResponse =
          objectMapper.readTree(waitForSearchResponseBody(driver, properties.responseUrlKeyword()));
    } catch (CaptureException exception) {
      throw exception;
    } catch (Exception exception) {
      throw new CaptureException(
          "invalid_response", "response_body", "Search response was not valid JSON", exception);
    }
    if (targetPage <= 1) {
      return extractFirstPageItems(firstPageResponse);
    }

    try {
      switchToSearchIframe(driver);
      return capturePaginatedItems(driver, targetPage);
    } catch (CaptureException exception) {
      throw exception;
    } catch (Exception exception) {
      throw captureFailure("pagination", exception);
    }
  }

  private JsonNode capturePaginatedItems(ChromeDriver driver, int targetPage) throws Exception {
    By targetSelector = By.linkText(String.valueOf(targetPage));
    WebDriverWait wait = new WebDriverWait(driver, SEARCH_RESPONSE_TIMEOUT);
    wait.ignoring(StaleElementReferenceException.class);

    // Naver renders these links on the server before React hydrates them. A
    // merely visible/clickable link follows its href and reloads page 1.
    wait.until(
        ignored -> {
          WebElement candidate =
              ExpectedConditions.elementToBeClickable(By.linkText("1")).apply(driver);
          if (candidate == null) {
            return null;
          }
          boolean hasClickHandler =
              Boolean.TRUE.equals(
                  driver.executeScript(
                      "const el=arguments[0]; return Object.keys(el).some(key => "
                          + "(key.startsWith('__reactProps$') || key.startsWith('__reactEventHandlers$')) "
                          + "&& typeof el[key].onClick === 'function');",
                      candidate));
          return hasClickHandler ? candidate : null;
        });
    if (driver.findElements(targetSelector).isEmpty()) {
      LOGGER.info("Naver has no result page {}", targetPage);
      return objectMapper.createArrayNode();
    }
    String selectedClass = driver.findElement(By.linkText("1")).getAttribute("class");
    LOGGER.info("Naver pagination controls ready for page {}", targetPage);
    for (int attempt = 0; attempt < 2; attempt++) {
      clearPerformanceLogs(driver);
      driver.findElement(targetSelector).click();
      wait.until(
          ignored ->
              selectedClass.equals(driver.findElement(targetSelector).getAttribute("class")));
      LOGGER.info("Naver pagination selected page {}", targetPage);
      try {
        return extractGraphqlItems(
            parseGraphqlResponse(
                waitForSearchResponseBody(
                    driver, properties.graphqlResponseUrlKeyword(), targetPage)));
      } catch (CaptureException exception) {
        if (attempt > 0 || !"request_aborted".equals(exception.category())) {
          throw exception;
        }
        // An aborted fetch cannot produce a body. Re-select once in the same
        // initialized browser; never return the old page's cached results.
        LOGGER.warn("Naver aborted the page {} request; re-selecting once", targetPage);
        By firstPageSelector = By.linkText("1");
        driver.findElement(firstPageSelector).click();
        wait.until(
            ignored ->
                selectedClass.equals(driver.findElement(firstPageSelector).getAttribute("class")));
      }
    }
    throw new IllegalStateException("Pagination capture did not complete");
  }

  private JsonNode parseGraphqlResponse(String responseBody) {
    try {
      return objectMapper.readTree(responseBody);
    } catch (Exception exception) {
      throw new CaptureException(
          "invalid_response", "response_body", "Paginated response was not valid JSON", exception);
    }
  }

  private void switchToSearchIframe(ChromeDriver driver) {
    driver.switchTo().defaultContent();
    WebDriverWait wait = new WebDriverWait(driver, ELEMENT_WAIT_TIMEOUT);
    wait.until(
        ExpectedConditions.frameToBeAvailableAndSwitchToIt(By.cssSelector("iframe#searchIframe")));
  }

  private void clearPerformanceLogs(ChromeDriver driver) {
    driver.manage().logs().get(LogType.PERFORMANCE);
  }

  private JsonNode extractFirstPageItems(JsonNode response) {
    JsonNode list = response.path("result").path("place").path("list");
    if (list.isArray()) {
      return list;
    }

    throw new CaptureException(
        "invalid_response", "response_body", "Search response did not contain a result list", null);
  }

  private JsonNode extractGraphqlItems(JsonNode response) {
    JsonNode items = findItemsNode(response.path("data"));
    if (items != null) {
      return items;
    }

    if (response.isArray()) {
      JsonNode firstEmptyItems = null;
      for (JsonNode payload : response) {
        JsonNode candidate = findItemsNode(payload.path("data"));
        if (candidate == null) {
          continue;
        }
        if (!candidate.isEmpty()) {
          return candidate;
        }
        if (firstEmptyItems == null) {
          firstEmptyItems = candidate;
        }
      }
      if (firstEmptyItems != null) {
        return firstEmptyItems;
      }
    }

    throw new CaptureException(
        "invalid_response",
        "response_body",
        "Paginated response did not contain result items",
        null);
  }

  private JsonNode findItemsNode(JsonNode node) {
    if (node == null || node.isMissingNode() || node.isNull()) {
      return null;
    }

    if (node.isObject()) {
      JsonNode directItems = node.get("items");
      if (directItems != null && directItems.isArray()) {
        return directItems;
      }

      JsonNode firstEmptyItems = null;
      Iterator<JsonNode> children = node.elements();
      while (children.hasNext()) {
        JsonNode child = children.next();
        JsonNode candidate = findItemsNode(child);
        if (candidate == null) {
          continue;
        }
        if (!candidate.isEmpty()) {
          return candidate;
        }
        if (firstEmptyItems == null) {
          firstEmptyItems = candidate;
        }
      }
      return firstEmptyItems;
    }

    if (node.isArray()) {
      JsonNode firstEmptyItems = null;
      for (JsonNode child : node) {
        JsonNode candidate = findItemsNode(child);
        if (candidate == null) {
          continue;
        }
        if (!candidate.isEmpty()) {
          return candidate;
        }
        if (firstEmptyItems == null) {
          firstEmptyItems = candidate;
        }
      }
      return firstEmptyItems;
    }

    return null;
  }

  private String waitForSearchResponseBody(ChromeDriver driver, String responseUrlKeyword)
      throws Exception {
    return waitForSearchResponseBody(driver, responseUrlKeyword, 0);
  }

  private String waitForSearchResponseBody(
      ChromeDriver driver, String responseUrlKeyword, int targetPage) throws Exception {
    Instant deadline = Instant.now().plus(SEARCH_RESPONSE_TIMEOUT);
    Set<String> finishedRequestIds = new HashSet<>();
    Set<String> failedRequestIds = new HashSet<>();
    Set<String> abortedRequestIds = new HashSet<>();
    Set<String> observedResponsePaths = new java.util.LinkedHashSet<>();
    Set<String> observedRequestPaths = new java.util.LinkedHashSet<>();
    Set<String> failedResponsePaths = new java.util.LinkedHashSet<>();
    Map<String, String> requestPaths = new java.util.HashMap<>();
    Map<String, ResponseCandidate> candidates = new java.util.LinkedHashMap<>();
    Map<String, Integer> matchingOperations = new java.util.HashMap<>();
    boolean retryUsed = false;
    int bodyReadFailures = 0;

    while (Instant.now().isBefore(deadline)) {
      LogEntries entries = driver.manage().logs().get(LogType.PERFORMANCE);
      List<LogEntry> logs = entries.getAll();
      for (LogEntry entry : logs) {
        JsonNode message;
        try {
          message = objectMapper.readTree(entry.getMessage()).path("message");
        } catch (Exception exception) {
          throw new CaptureException(
              "capture_error",
              "response_matching",
              "Could not read browser network events",
              exception);
        }
        String method = message.path("method").asText();
        JsonNode params = message.path("params");
        String requestId = params.path("requestId").asText();
        if ("Network.requestWillBeSent".equals(method)) {
          String requestUrl = params.path("request").path("url").asText();
          if (targetPage > 0 && requestUrl.contains(responseUrlKeyword)) {
            JsonNode payload =
                objectMapper.readTree(params.path("request").path("postData").asText("null"));
            int index = matchingPageOperation(payload, targetPage);
            if (index >= 0) {
              matchingOperations.put(requestId, index);
              LOGGER.info("Observed GraphQL request for page {}", targetPage);
            }
          }
          if (!requestId.isBlank() && !requestUrl.isBlank()) {
            requestPaths.put(requestId, safeResponsePath(requestUrl));
          }
          if (!requestUrl.isBlank()
              && isDynamicEndpoint(requestUrl)
              && observedRequestPaths.size() < 80) {
            observedRequestPaths.add(safeResponsePath(requestUrl));
          }
        } else if ("Network.responseReceived".equals(method)) {
          String url = params.path("response").path("url").asText();
          if (!url.isBlank()) {
            String responsePath = safeResponsePath(url);
            String normalizedResponsePath = responsePath.toLowerCase(java.util.Locale.ROOT);
            if ((normalizedResponsePath.contains("graphql")
                    || normalizedResponsePath.contains("/api/")
                    || normalizedResponsePath.contains("search"))
                && observedResponsePaths.size() < 50) {
              observedResponsePaths.add(
                  responsePath + " status=" + params.path("response").path("status").asInt(-1));
            }
          }
          if (!requestId.isBlank()
              && url.contains(responseUrlKeyword)
              && (targetPage == 0 || matchingOperations.containsKey(requestId))) {
            double status = params.path("response").path("status").asDouble(-1);
            candidates.put(requestId, new ResponseCandidate(status));
          }
        } else if ("Network.loadingFinished".equals(method)) {
          finishedRequestIds.add(requestId);
        } else if ("Network.loadingFailed".equals(method)) {
          failedRequestIds.add(requestId);
          if ("net::ERR_ABORTED".equals(params.path("errorText").asText())) {
            abortedRequestIds.add(requestId);
          }
          if (failedResponsePaths.size() < 20) {
            failedResponsePaths.add(
                requestPaths.getOrDefault(requestId, "<unknown-path>")
                    + ": "
                    + params.path("errorText").asText("unknown network error")
                    + ", type="
                    + params.path("type").asText("unknown")
                    + ", blockedReason="
                    + params.path("blockedReason").asText("none"));
          }
        }
      }

      if (targetPage > 0
          && !matchingOperations.isEmpty()
          && abortedRequestIds.containsAll(matchingOperations.keySet())) {
        throw new CaptureException(
            "request_aborted", "response_matching", "Naver aborted the requested page fetch", null);
      }

      for (Map.Entry<String, ResponseCandidate> entry : candidates.entrySet()) {
        String requestId = entry.getKey();
        ResponseCandidate candidate = entry.getValue();
        candidate.finished = finishedRequestIds.contains(requestId);
        candidate.failed = failedRequestIds.contains(requestId);
        if (!candidate.finished || candidate.failed || candidate.bodyReadAttempted) {
          continue;
        }
        if (candidate.status < 200 || candidate.status >= 300) {
          continue;
        }

        String bodyText;
        try {
          bodyText = readResponseBody(driver, requestId);
        } catch (WebDriverException exception) {
          bodyReadFailures++;
          if (!retryUsed && isTransientBodyReadFailure(exception)) {
            retryUsed = true;
            try {
              bodyText = readResponseBody(driver, requestId);
            } catch (WebDriverException retryException) {
              bodyReadFailures++;
              candidate.bodyReadAttempted = true;
              continue;
            }
          } else {
            candidate.bodyReadAttempted = true;
            continue;
          }
        }
        if (bodyText == null || bodyText.isBlank()) {
          bodyReadFailures++;
          candidate.bodyReadAttempted = true;
          continue;
        }
        if (targetPage > 0) {
          JsonNode response = objectMapper.readTree(bodyText);
          if (response.isArray()) {
            JsonNode operationResponse = response.get(matchingOperations.get(requestId));
            if (operationResponse == null) {
              throw new CaptureException(
                  "invalid_response",
                  "response_body",
                  "Paginated response did not match the requested operation",
                  null);
            }
            return operationResponse.toString();
          }
        }
        return bodyText;
      }

      try {
        Thread.sleep(200L);
      } catch (InterruptedException exception) {
        Thread.currentThread().interrupt();
        throw new CaptureException(
            "cancelled",
            "response_matching",
            "Waiting for search response was interrupted",
            exception);
      }
    }

    long finishedCount =
        candidates.values().stream().filter(candidate -> candidate.finished).count();
    long failedCount = candidates.values().stream().filter(candidate -> candidate.failed).count();
    boolean hasBodyCandidate =
        candidates.values().stream()
            .anyMatch(
                candidate ->
                    candidate.finished
                        && !candidate.failed
                        && candidate.status >= 200
                        && candidate.status < 300);
    String failureStage = hasBodyCandidate ? "response_body" : "response_matching";
    LOGGER.warn(
        "Naver response capture timed out: expectedPath={}, matched={}, finished={}, failed={}, bodyReadFailures={}, observedRequests={}, observedPaths={}, networkFailures={}",
        responseUrlKeyword,
        candidates.size(),
        finishedCount,
        failedCount,
        bodyReadFailures,
        observedRequestPaths,
        observedResponsePaths,
        failedResponsePaths);
    throw new CaptureException(
        "timeout", failureStage, "Timed out while waiting for Naver search response", null);
  }

  private int matchingPageOperation(JsonNode payload, int targetPage) {
    if (payload == null) {
      return -1;
    }
    if (!payload.isArray()) {
      return hasPageOffset(payload.path("variables"), targetPage) ? 0 : -1;
    }
    for (int index = 0; index < payload.size(); index++) {
      if (hasPageOffset(payload.get(index).path("variables"), targetPage)) {
        return index;
      }
    }
    return -1;
  }

  private boolean hasPageOffset(JsonNode node, int targetPage) {
    int display = node.path("display").asInt(0);
    if (display > 0 && node.path("start").asInt(-1) == (targetPage - 1) * display + 1) {
      return true;
    }
    for (JsonNode child : node) {
      if (hasPageOffset(child, targetPage)) {
        return true;
      }
    }
    return false;
  }

  private String safeResponsePath(String url) {
    try {
      URI uri = URI.create(url);
      String path = uri.getPath();
      if (path == null || path.isBlank()) {
        path = "/";
      } else if (path.startsWith("/p/search/")) {
        path = "/p/search/{query}";
      }
      String host = uri.getHost();
      return (host == null ? "<unknown-host>" : host) + path;
    } catch (IllegalArgumentException exception) {
      return "<unparseable-url>";
    }
  }

  private boolean isDynamicEndpoint(String url) {
    try {
      String path = URI.create(url).getPath();
      if (path == null || path.isBlank()) {
        return false;
      }
      String normalizedPath = path.toLowerCase(java.util.Locale.ROOT);
      return !normalizedPath.contains("/assets/")
          && !normalizedPath.contains("/resource/api/v2/image/")
          && !normalizedPath.matches(".*\\.(js|css|png|jpe?g|webp|svg|woff2?|ttf|ico)$");
    } catch (IllegalArgumentException exception) {
      return false;
    }
  }

  private String readResponseBody(ChromeDriver driver, String requestId) {
    Map<String, Object> bodyResult =
        driver.executeCdpCommand("Network.getResponseBody", Map.of("requestId", requestId));
    Object body = bodyResult.get("body");
    if (!(body instanceof String bodyText)) {
      return null;
    }
    if (Boolean.TRUE.equals(bodyResult.get("base64Encoded"))) {
      return new String(Base64.getDecoder().decode(bodyText), StandardCharsets.UTF_8);
    }
    return bodyText;
  }

  private boolean isTransientBodyReadFailure(WebDriverException exception) {
    String message = exception.getMessage();
    return message != null && message.toLowerCase().contains("no resource with given identifier");
  }

  private static final class ResponseCandidate {
    private final double status;
    private boolean finished;
    private boolean failed;
    private boolean bodyReadAttempted;

    private ResponseCandidate(double status) {
      this.status = status;
    }
  }
}
