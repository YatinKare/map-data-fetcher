package com.example.mapdatafetcher.controller;

import com.example.mapdatafetcher.dto.HealthResponse;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class HealthController {

  @GetMapping(value = "/healthz", produces = MediaType.APPLICATION_JSON_VALUE)
  public HealthResponse health() {
    return new HealthResponse("ok");
  }
}
