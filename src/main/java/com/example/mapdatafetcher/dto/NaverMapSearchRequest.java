package com.example.mapdatafetcher.dto;

import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;

public record NaverMapSearchRequest(
    @NotBlank(message = "q is required") String q,
    @Min(value = 1, message = "page must be at least 1")
        @Max(value = 5, message = "page must be at most 5")
        Integer page) {
  public NaverMapSearchRequest {
    page = page == null ? 1 : page;
  }
}
