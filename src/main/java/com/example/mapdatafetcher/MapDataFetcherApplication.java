package com.example.mapdatafetcher;

import com.example.mapdatafetcher.config.NaverMapSeleniumProperties;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.EnableConfigurationProperties;

@SpringBootApplication
@EnableConfigurationProperties(NaverMapSeleniumProperties.class)
public class MapDataFetcherApplication {

  public static void main(String[] args) {
    SpringApplication.run(MapDataFetcherApplication.class, args);
  }
}
