package com.example.mapdatafetcher.controller;

import com.example.mapdatafetcher.dto.NaverMapCoordinateSearchRequest;
import com.example.mapdatafetcher.dto.NaverMapSearchRequest;
import com.example.mapdatafetcher.service.NaverMapSearchService;
import com.fasterxml.jackson.databind.JsonNode;
import jakarta.validation.Valid;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.ModelAttribute;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/naver-map")
public class NaverMapSearchController {

  private final NaverMapSearchService naverMapSearchService;

  public NaverMapSearchController(NaverMapSearchService naverMapSearchService) {
    this.naverMapSearchService = naverMapSearchService;
  }

  @GetMapping(value = "/search", produces = MediaType.APPLICATION_JSON_VALUE)
  public JsonNode search(@Valid @ModelAttribute NaverMapSearchRequest request) {
    return naverMapSearchService.search(request);
  }

  @GetMapping(value = "/coordinate", produces = MediaType.APPLICATION_JSON_VALUE)
  public JsonNode searchByCoordinate(
      @Valid @ModelAttribute NaverMapCoordinateSearchRequest request) {
    return naverMapSearchService.searchByCoordinate(request);
  }
}
