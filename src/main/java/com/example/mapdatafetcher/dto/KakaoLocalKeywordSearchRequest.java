package com.example.mapdatafetcher.dto;

import jakarta.validation.constraints.DecimalMax;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;

public record KakaoLocalKeywordSearchRequest(
    @NotBlank(message = "query is required") String query,
    @NotNull(message = "x is required") @DecimalMin(value = "-180.0", message = "x must be at least -180")
        @DecimalMax(value = "180.0", message = "x must be at most 180")
        Double x,
    @NotNull(message = "y is required") @DecimalMin(value = "-90.0", message = "y must be at least -90")
        @DecimalMax(value = "90.0", message = "y must be at most 90")
        Double y,
    @Min(value = 0, message = "radius must be at least 0")
        @Max(value = 20000, message = "radius must be at most 20000")
        Integer radius,
    @Min(value = 1, message = "page must be at least 1")
        @Max(value = 45, message = "page must be at most 45")
        Integer page) {
  public KakaoLocalKeywordSearchRequest {
    page = page == null ? 1 : page;
  }
}
