package com.example.mapdatafetcher.service;

import com.example.mapdatafetcher.config.NaverMapSeleniumProperties;
import com.example.mapdatafetcher.dto.NaverMapCoordinateSearchRequest;
import com.example.mapdatafetcher.dto.NaverMapSearchRequest;
import com.example.mapdatafetcher.exception.CaptureException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.Base64;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.logging.Level;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.openqa.selenium.By;
import org.openqa.selenium.Keys;
import org.openqa.selenium.PageLoadStrategy;
import org.openqa.selenium.TimeoutException;
import org.openqa.selenium.WebDriver;
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
  private static final Pattern NUMERIC_RANK = Pattern.compile("\\d+");
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
        "--window-size=1920,1080");

    if (properties.headless()) {
      options.addArguments("--headless=new");
    }

    LoggingPreferences loggingPreferences = new LoggingPreferences();
    loggingPreferences.enable(LogType.PERFORMANCE, Level.ALL);
    options.setCapability("goog:loggingPrefs", loggingPreferences);

    ChromeDriver driver = new ChromeDriver(options);
    driver.manage().timeouts().pageLoadTimeout(PAGE_LOAD_TIMEOUT);
    driver.manage().window().maximize();
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
      return extractGraphqlItems(captureGraphqlByPage(driver, targetPage));
    } catch (CaptureException exception) {
      throw exception;
    } catch (Exception exception) {
      throw captureFailure("pagination", exception);
    }
  }

  private JsonNode navigateToPageAndCaptureGraphql(ChromeDriver driver, int targetPage)
      throws Exception {
    WebDriverWait wait = new WebDriverWait(driver, ELEMENT_WAIT_TIMEOUT);
    By paginationContainerSelector = By.xpath("//*[@id='app-root']/div/div[2]/div[2]");
    By pageButtonSelector =
        By.xpath("//*[@id='app-root']/div/div[2]/div[2]/a[normalize-space(text()) != '']");

    try {
      wait.until(ExpectedConditions.visibilityOfElementLocated(paginationContainerSelector));
    } catch (TimeoutException exception) {
      throw new IllegalStateException("Pagination bar did not appear", exception);
    }

    Optional<WebElement> targetButton = findPageButton(driver, targetPage, pageButtonSelector);
    if (targetButton.isEmpty()) {
      throw new IllegalStateException("Requested page button not found: " + targetPage);
    }

    clearPerformanceLogs(driver);
    WebElement clickableButton;
    try {
      clickableButton = wait.until(ExpectedConditions.elementToBeClickable(targetButton.get()));
    } catch (TimeoutException exception) {
      throw new IllegalStateException(
          "Target page button was not clickable: " + targetPage, exception);
    }

    clickableButton.click();
    try {
      wait.until(
          driverInstance ->
              findSelectedPageButton(driverInstance, pageButtonSelector)
                  .map(button -> String.valueOf(targetPage).equals(button.getText().trim()))
                  .orElse(false));
    } catch (TimeoutException exception) {
      throw new IllegalStateException(
          "Page selection did not change after click: " + targetPage, exception);
    }

    return parseGraphqlResponse(
        waitForSearchResponseBody(driver, properties.graphqlResponseUrlKeyword()));
  }

  private JsonNode parseGraphqlResponse(String responseBody) {
    try {
      return objectMapper.readTree(responseBody);
    } catch (Exception exception) {
      throw new CaptureException(
          "invalid_response", "response_body", "Paginated response was not valid JSON", exception);
    }
  }

  private JsonNode captureGraphqlByPage(ChromeDriver driver, int targetPage) throws Exception {
    if (targetPage <= 1) {
      navigateToPageAndCaptureGraphql(driver, 2);
      return navigateToPageAndCaptureGraphql(driver, 1);
    }

    return navigateToPageAndCaptureGraphql(driver, targetPage);
  }

  private void switchToSearchIframe(ChromeDriver driver) {
    driver.switchTo().defaultContent();
    WebDriverWait wait = new WebDriverWait(driver, ELEMENT_WAIT_TIMEOUT);
    wait.until(
        ExpectedConditions.frameToBeAvailableAndSwitchToIt(By.cssSelector("iframe#searchIframe")));
  }

  private Optional<WebElement> findPageButton(
      WebDriver driver, int targetPage, By pageButtonSelector) {
    return driver.findElements(pageButtonSelector).stream()
        .filter(element -> String.valueOf(targetPage).equals(element.getText().trim()))
        .findFirst();
  }

  private Optional<WebElement> findSelectedPageButton(WebDriver driver, By pageButtonSelector) {
    return driver.findElements(pageButtonSelector).stream()
        .filter(
            element -> {
              String className = element.getAttribute("class");
              String ariaCurrent = element.getAttribute("aria-current");
              return (className != null && className.contains("qxokY"))
                  || "page".equalsIgnoreCase(ariaCurrent);
            })
        .findFirst();
  }

  private void clearPerformanceLogs(ChromeDriver driver) {
    driver.manage().logs().get(LogType.PERFORMANCE);
  }

  private JsonNode extractFirstPageItems(JsonNode response) {
    JsonNode list = response.path("result").path("place").path("list");
    if (list.isArray()) {
      return normalizeItems(list);
    }

    throw new CaptureException(
        "invalid_response", "response_body", "Search response did not contain a result list", null);
  }

  private JsonNode extractGraphqlItems(JsonNode response) {
    JsonNode items = findItemsNode(response.path("data"));
    if (items != null) {
      return normalizeItems(items);
    }

    if (response.isArray()) {
      JsonNode firstEmptyItems = null;
      for (JsonNode payload : response) {
        JsonNode candidate = findItemsNode(payload.path("data"));
        if (candidate == null) {
          continue;
        }
        if (!candidate.isEmpty()) {
          return normalizeItems(candidate);
        }
        if (firstEmptyItems == null) {
          firstEmptyItems = candidate;
        }
      }
      if (firstEmptyItems != null) {
        return normalizeItems(firstEmptyItems);
      }
    }

    throw new CaptureException(
        "invalid_response",
        "response_body",
        "Paginated response did not contain result items",
        null);
  }

  private ArrayNode normalizeItems(JsonNode items) {
    ArrayNode normalized = objectMapper.createArrayNode();
    for (JsonNode item : items) {
      if (item.isObject()) {
        normalized.add(normalizeItem(item));
      }
    }
    return normalized;
  }

  private ObjectNode normalizeItem(JsonNode item) {
    ObjectNode result = objectMapper.createObjectNode();

    String rank = textValue(item, "rank");
    if (rank != null) {
      Matcher rankMatcher = NUMERIC_RANK.matcher(rank);
      if (rankMatcher.matches()) {
        result.put("rank", Integer.parseInt(rank));
      }
    }
    copyText(item, result, "id", "id");
    copyText(item, result, "name", "name");
    JsonNode category = item.get("category");
    if (category != null && !category.isNull() && !category.isEmpty()) {
      result.set("category", category.deepCopy());
    }
    copyText(item, result, "roadAddress", "road_address");

    ObjectNode coordinates = objectMapper.createObjectNode();
    putCoordinate(item, coordinates, "x", "longitude");
    putCoordinate(item, coordinates, "y", "latitude");
    if (!coordinates.isEmpty()) {
      result.set("coordinates", coordinates);
    }

    copyFirstText(item, result, "tel", "tel", "virtualTel");
    copyText(item, result, "thumUrl", "thumbnail_url");
    copyText(item, result, "homePage", "homepage");
    copyText(item, result, "menuInfo", "menu_info");
    addBusinessInformation(item, result);
    addReservationOptions(item, result);
    return result;
  }

  private void addBusinessInformation(JsonNode item, ObjectNode result) {
    JsonNode businessStatus = item.path("businessStatus");
    JsonNode status = businessStatus.path("status");
    String statusText = firstNonBlank(textValue(status, "text"), textValue(status, "description"));
    if (statusText != null) {
      result.put("business_status", statusText);
    }

    String businessHours = formatTimeRange(textValue(businessStatus, "businessHours"));
    if (businessHours != null) {
      result.put("business_hours", businessHours);
    }
    String breakTime = formatTimeRange(textValue(businessStatus, "breakTime"));
    if (breakTime != null) {
      result.put("break_time", breakTime);
    }
    String lastOrder = formatTime(textValue(businessStatus, "lastOrder"));
    if (lastOrder != null) {
      result.put("last_order", lastOrder);
    }
  }

  private void addReservationOptions(JsonNode item, ObjectNode result) {
    JsonNode labels = item.path("reservationLabel");
    ArrayNode options = objectMapper.createArrayNode();
    addReservationOption(labels, options, "standard", "reservation");
    addReservationOption(labels, options, "preOrder", "pre-order");
    addReservationOption(labels, options, "table", "table");
    addReservationOption(labels, options, "takeout", "takeout");
    if (!options.isEmpty()) {
      result.set("reservation_options", options);
    }
  }

  private void addReservationOption(
      JsonNode labels, ArrayNode options, String sourceField, String outputValue) {
    if (labels.path(sourceField).asBoolean(false)) {
      options.add(outputValue);
    }
  }

  private void putCoordinate(
      JsonNode item, ObjectNode coordinates, String sourceField, String outputField) {
    String value = textValue(item, sourceField);
    if (value == null) {
      return;
    }
    try {
      coordinates.put(outputField, Double.parseDouble(value));
    } catch (NumberFormatException ignored) {
      // Ignore malformed upstream coordinates instead of passing through unstable values.
    }
  }

  private void copyText(
      JsonNode source, ObjectNode destination, String sourceField, String outputField) {
    String value = textValue(source, sourceField);
    if (value != null) {
      destination.put(outputField, value);
    }
  }

  private void copyFirstText(
      JsonNode source, ObjectNode destination, String outputField, String... sourceFields) {
    for (String sourceField : sourceFields) {
      String value = textValue(source, sourceField);
      if (value != null) {
        destination.put(outputField, value);
        return;
      }
    }
  }

  private String textValue(JsonNode source, String field) {
    JsonNode value = source.get(field);
    if (value == null || value.isNull()) {
      return null;
    }
    String text = value.asText().trim();
    return text.isEmpty() ? null : text;
  }

  private String firstNonBlank(String... values) {
    for (String value : values) {
      if (value != null && !value.isBlank()) {
        return value;
      }
    }
    return null;
  }

  private String formatTimeRange(String value) {
    if (value == null) {
      return null;
    }
    String[] parts = value.split("~", -1);
    if (parts.length != 2) {
      return null;
    }
    String start = formatTime(parts[0]);
    String end = formatTime(parts[1]);
    return start == null || end == null ? null : start + "–" + end;
  }

  private String formatTime(String value) {
    if (value == null) {
      return null;
    }
    String digits = value.replaceAll("\\D", "");
    if (digits.length() == 12) {
      digits = digits.substring(8);
    }
    if (digits.length() != 4) {
      return null;
    }
    int hours = Integer.parseInt(digits.substring(0, 2));
    int minutes = Integer.parseInt(digits.substring(2));
    if (hours > 24 || minutes > 59 || (hours == 24 && minutes != 0)) {
      return null;
    }
    return digits.substring(0, 2) + ":" + digits.substring(2);
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
    Instant deadline = Instant.now().plus(SEARCH_RESPONSE_TIMEOUT);
    Set<String> finishedRequestIds = new HashSet<>();
    Set<String> failedRequestIds = new HashSet<>();
    Map<String, ResponseCandidate> candidates = new java.util.LinkedHashMap<>();
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
        if ("Network.responseReceived".equals(method)) {
          String url = params.path("response").path("url").asText();
          if (!requestId.isBlank() && url.contains(responseUrlKeyword)) {
            double status = params.path("response").path("status").asDouble(-1);
            candidates.put(requestId, new ResponseCandidate(status));
          }
        } else if ("Network.loadingFinished".equals(method)) {
          finishedRequestIds.add(requestId);
        } else if ("Network.loadingFailed".equals(method)) {
          failedRequestIds.add(requestId);
        }
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
        "Naver response capture timed out: matched={}, finished={}, failed={}, bodyReadFailures={}",
        candidates.size(),
        finishedCount,
        failedCount,
        bodyReadFailures);
    throw new CaptureException(
        "timeout", failureStage, "Timed out while waiting for Naver search response", null);
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
