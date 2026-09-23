package com.example.mapdatafetcher.dto;

import java.util.Map;

public record ValidationErrorResponse(String message, Map<String, String> errors) {}
