package com.example.mapdatafetcher.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "naver.map.selenium")
public record NaverMapSeleniumProperties(
    String searchUrl,
    String responseUrlKeyword,
    String graphqlResponseUrlKeyword,
    boolean headless) {}
