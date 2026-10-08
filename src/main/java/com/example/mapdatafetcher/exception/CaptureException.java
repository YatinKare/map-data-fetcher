package com.example.mapdatafetcher.exception;

public class CaptureException extends RuntimeException {

  private final String category;
  private final String stage;

  public CaptureException(String category, String stage, String message, Throwable cause) {
    super(message, cause);
    this.category = category;
    this.stage = stage;
  }

  public String category() {
    return category;
  }

  public String stage() {
    return stage;
  }
}
