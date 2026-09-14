package com.example.mapdatafetcher.service;

import com.example.mapdatafetcher.config.NaverMapSeleniumProperties;
import com.example.mapdatafetcher.dto.NaverMapCoordinateSearchRequest;
import com.example.mapdatafetcher.dto.NaverMapSearchRequest;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.Base64;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.logging.Level;
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
  private static final Duration SEARCH_RESPONSE_TIMEOUT = Duration.ofSeconds(30);
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
    ChromeDriver driver = createDriver();
    try {
      driver.executeCdpCommand("Network.enable", Map.of());
      Integer requestedPage = request.page();
      int targetPage = requestedPage == null ? 1 : requestedPage;
      driver.get(buildSearchUrl(request.q()));
      JsonNode result = captureSearchResults(driver, targetPage);
      LOGGER.info("Naver keyword search completed for page {}", targetPage);
      return result;
    } catch (Exception exception) {
      LOGGER.error("Naver keyword search failed", exception);
      throw new IllegalStateException("Failed to capture Naver map search response", exception);
    } finally {
      driver.quit();
    }
  }

  public JsonNode searchByCoordinate(NaverMapCoordinateSearchRequest request) {
    LOGGER.info("Starting Naver coordinate search for page {}", request.page());
    ChromeDriver driver = createDriver();
    try {
      driver.executeCdpCommand("Network.enable", Map.of());
      Integer requestedPage = request.page();
      int targetPage = requestedPage == null ? 1 : requestedPage;
      driver.get(buildCoordinateUrl(request.x(), request.y()));
      submitSearchKeyword(driver, request.query());
      JsonNode result = captureSearchResults(driver, targetPage);
      LOGGER.info("Naver coordinate search completed for page {}", targetPage);
      return result;
    } catch (Exception exception) {
      LOGGER.error("Naver coordinate search failed", exception);
      throw new IllegalStateException("Failed to capture Naver map search response", exception);
    } finally {
      driver.quit();
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

  private String buildCoordinateUrl(Double x, Double y) {
    String searchUrl = properties.searchUrl();
    int searchPathIndex = searchUrl.indexOf("/search/");
    String baseUrl = searchPathIndex >= 0 ? searchUrl.substring(0, searchPathIndex) : searchUrl;
    return UriComponentsBuilder.fromUriString(baseUrl)
        .queryParam("lng", x)
        .queryParam("lat", y)
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
    JsonNode firstPageResponse =
        objectMapper.readTree(waitForSearchResponseBody(driver, properties.responseUrlKeyword()));
    if (targetPage <= 1) {
      return extractFirstPageItems(firstPageResponse);
    }

    switchToSearchIframe(driver);
    return extractGraphqlItems(captureGraphqlByPage(driver, targetPage));
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

    return objectMapper.readTree(
        waitForSearchResponseBody(driver, properties.graphqlResponseUrlKeyword()));
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
      return list;
    }

    return JsonNodeFactory.instance.arrayNode();
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

    return JsonNodeFactory.instance.arrayNode();
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

    while (Instant.now().isBefore(deadline)) {
      LogEntries entries = driver.manage().logs().get(LogType.PERFORMANCE);
      List<LogEntry> logs = entries.getAll();
      for (LogEntry entry : logs) {
        JsonNode message = objectMapper.readTree(entry.getMessage()).path("message");
        String method = message.path("method").asText();
        if (!"Network.responseReceived".equals(method)) {
          continue;
        }

        JsonNode params = message.path("params");
        String url = params.path("response").path("url").asText();
        if (!url.contains(responseUrlKeyword)) {
          continue;
        }

        String requestId = params.path("requestId").asText();
        Map<String, Object> bodyResult;
        try {
          bodyResult =
              driver.executeCdpCommand("Network.getResponseBody", Map.of("requestId", requestId));
        } catch (WebDriverException ignored) {
          continue;
        }
        Object body = bodyResult.get("body");
        if (!(body instanceof String bodyText)) {
          break;
        }

        if (Boolean.TRUE.equals(bodyResult.get("base64Encoded"))) {
          return new String(Base64.getDecoder().decode(bodyText), StandardCharsets.UTF_8);
        }

        return bodyText;
      }

      Thread.sleep(200L);
    }

    throw new TimeoutException("Timed out while waiting for Naver search response");
  }
}
