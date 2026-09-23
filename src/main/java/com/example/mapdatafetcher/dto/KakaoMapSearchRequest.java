package com.example.mapdatafetcher.dto;

import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;

public record KakaoMapSearchRequest(
    @NotBlank(message = "q is required") String q,
    @Min(value = 1, message = "page must be at least 1")
        @Max(value = 500, message = "page must be at most 500")
        Integer page) {
  public KakaoMapSearchRequest {
    page = page == null ? 1 : page;
  }
}
