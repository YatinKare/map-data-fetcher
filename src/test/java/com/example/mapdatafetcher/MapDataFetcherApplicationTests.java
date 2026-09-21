package com.example.mapdatafetcher;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.client.TestRestTemplate;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class MapDataFetcherApplicationTests {

  @LocalServerPort private int port;

  @Autowired private TestRestTemplate restTemplate;

  @Test
  void healthEndpointIsAvailableFromRunningApplication() {
    ResponseEntity<String> response =
        restTemplate.getForEntity("http://127.0.0.1:" + port + "/healthz", String.class);

    assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
    assertThat(response.getBody()).contains("\"status\":\"ok\"");
  }
}
