package com.example.mapdatafetcher.controller;

import com.example.mapdatafetcher.dto.KakaoLocalKeywordSearchRequest;
import com.example.mapdatafetcher.dto.KakaoMapSearchRequest;
import com.example.mapdatafetcher.service.KakaoMapSearchService;
import com.fasterxml.jackson.databind.JsonNode;
import jakarta.validation.Valid;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.ModelAttribute;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/kakao-map")
public class KakaoMapSearchController {

  private final KakaoMapSearchService kakaoMapSearchService;

  public KakaoMapSearchController(KakaoMapSearchService kakaoMapSearchService) {
    this.kakaoMapSearchService = kakaoMapSearchService;
  }

  @GetMapping(value = "/search", produces = MediaType.APPLICATION_JSON_VALUE)
  public JsonNode search(@Valid @ModelAttribute KakaoMapSearchRequest request) {
    return kakaoMapSearchService.search(request);
  }

  @GetMapping(value = "/coordinate", produces = MediaType.APPLICATION_JSON_VALUE)
  public JsonNode searchLocalKeyword(
      @Valid @ModelAttribute KakaoLocalKeywordSearchRequest request) {
    return kakaoMapSearchService.searchLocalKeyword(request);
  }
}
