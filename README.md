## [Velog - 서버 환경 설정 정리](https://velog.io/@0verfl0w767/Spring-Boot-%EC%84%9C%EB%B2%84%EC%97%90-%EC%84%A4%EC%B9%98)

## 서버 환경
- **Cloud & OS** : GCP Compute Engine (E2), Ubuntu 26.04
- **Backend** : Java 17, Spring Boot 3.3.8
  - Kakao Map API : Spring RestClient, Kakao Map Keyword API
  - Naver Map API : Selenium WebDriver(ChromeDriver), Chrome DevTools Protocol 기반 네트워크 응답 수집
  - Request/Response : Spring MVC, Jackson
- **API Docs** : Swagger UI, OpenAPI 3 (springdoc-openapi)
- **Infra & Process Manager** : Nginx, PM2
- **CI/CD** : GitHub Actions (scp-action, ssh-action)
- **Code Quality** : Spotless, google-java-format

## 배포 파이프라인
1. `main` 브랜치 Push 시 GitHub Actions 워크플로우 실행
2. 가상 환경(Ubuntu)에서 JDK 17 세팅 후 `./gradlew clean bootJar` 로 JAR 파일 빌드
3. **SCP**: `appleboy/scp-action`을 통해 빌드된 JAR 파일을 GCP 서버(`~/map-data-fetcher/build/libs`)로 전송
4. **SSH**: `appleboy/ssh-action`을 통해 서버에 SSH로 접속하여 PM2로 애플리케이션 시작 또는 재시작 (`pm2 start` / `pm2 restart`)


## Swagger API 문서

| Method | Path | Summary | Tag |
| --- | --- | --- | --- |
| GET | /api/naver-map/search | 네이버맵 검색 | 네이버맵 |
| GET | /api/naver-map/coordinate | 좌표 기준 네이버맵 검색 | 네이버맵 |
| GET | /api/kakao-map/search | 카카오맵 검색 | 카카오맵 |
| GET | /api/kakao-map/coordinate | 좌표 기준 카카오맵 검색 | 카카오맵 |

### 네이버맵

#### GET /api/naver-map/coordinate

좌표 기준 네이버맵 검색

검색어와 중심 좌표(경도, 위도)를 기준으로 네이버맵 검색 결과 원본 데이터를 조회합니다.

##### Sample Request

```http
GET /api/naver-map/coordinate?query=%EC%B9%B4%ED%8E%98&x=127.105649&y=37.64349&page=1
```

##### Parameters

| Name | In | Required | Type | Description | Example |
| --- | --- | --- | --- | --- | --- |
| query | query | True | string | 검색어 | 카페 |
| x | query | True | string | 중심 좌표 경도(longitude) | 127.105649 |
| y | query | True | string | 중심 좌표 위도(latitude) | 37.64349 |
| page | query | False | string | 결과 페이지 번호 | 1 |

##### Responses

| Status | Description |
| --- | --- |
| 502 | 네이버맵 응답 수집에 실패했습니다 |
| 200 | 좌표 기반 검색에 성공했습니다 |
| 400 | 요청 파라미터가 올바르지 않습니다 |

##### Example Response (502)

```json
{
    "message":  "네이버맵 응답 수집에 실패했습니다",
    "detail":  "Timed out while waiting for Naver search response"
}
```

##### Example Response (200)

```json
[
    {
        "id":  "1225845295",
        "name":  "파스타한끼 노원본점",
        "category":  "스파게티,파스타전문",
        "roadAddress":  "상계로5길 19 대광빌딩 1층 파스타한끼노원본점",
        "x":  "127.0635998",
        "y":  "37.6569760",
        "distance":  "380m",
        "hasBooking":  true,
        "hasNPay":  true,
        "visitorReviewCount":  "4,819",
        "imageUrl":  "https://ldb-phinf.pstatic.net/20241128_37/1732779573101N4BfO_JPEG/%B0%A1%B0%D4%C0%CC%B9%CC%C1%F6_%BF%DC%BA%CE_01.jpg",
        "bookingUrl":  "https://m.booking.naver.com/booking/6/bizes/967012/search",
        "newBusinessHours":  {
                                 "status":  "영업 중",
                                 "description":  "21:00에 라스트오더"
                             }
    },
    {
        "id":  "2009662047",
        "name":  "코지하우스 노원점",
        "category":  "양식",
        "roadAddress":  "동일로 1419 1층",
        "x":  "127.0601125",
        "y":  "37.6552875",
        "distance":  "600m",
        "hasBooking":  null,
        "hasNPay":  false,
        "visitorReviewCount":  "1,481",
        "imageUrl":  "https://ldb-phinf.pstatic.net/20260219_231/1771466879338Dor9H_JPEG/KakaoTalk_20251229_111436052.jpg",
        "phone":  "02-936-6683",
        "newOpening":  true,
        "newBusinessHours":  {
                                 "status":  "브레이크타임",
                                 "description":  "17:00에 영업 시작"
                             }
    }
]
```

#### GET /api/naver-map/search

네이버맵 검색

검색어와 페이지 번호로 네이버맵 검색 결과 원본 데이터를 조회합니다.

##### Sample Request

```http
GET /api/naver-map/search?q=%EB%85%B8%EC%9B%90%EC%97%AD%20%EB%A7%9B%EC%A7%91&page=1
```

##### Parameters

| Name | In | Required | Type | Description | Example |
| --- | --- | --- | --- | --- | --- |
| q | query | True | string | 검색어 | 노원역 맛집 |
| page | query | False | string | 결과 페이지 번호 | 1 |

##### Responses

| Status | Description |
| --- | --- |
| 502 | 네이버맵 응답 수집에 실패했습니다 |
| 200 | 검색에 성공했습니다 |
| 400 | 요청 파라미터가 올바르지 않습니다 |

##### Example Response (502)

```json
{
    "message":  "네이버맵 응답 수집에 실패했습니다",
    "detail":  "Timed out while waiting for Naver search response"
}
```

##### Example Response (200)

```json
[
    {
        "id":  "1225845295",
        "name":  "파스타한끼 노원본점",
        "category":  "스파게티,파스타전문",
        "roadAddress":  "상계로5길 19 대광빌딩 1층 파스타한끼노원본점",
        "x":  "127.0635998",
        "y":  "37.6569760",
        "distance":  "380m",
        "hasBooking":  true,
        "hasNPay":  true,
        "visitorReviewCount":  "4,819",
        "imageUrl":  "https://ldb-phinf.pstatic.net/20241128_37/1732779573101N4BfO_JPEG/%B0%A1%B0%D4%C0%CC%B9%CC%C1%F6_%BF%DC%BA%CE_01.jpg",
        "bookingUrl":  "https://m.booking.naver.com/booking/6/bizes/967012/search",
        "newBusinessHours":  {
                                 "status":  "영업 중",
                                 "description":  "21:00에 라스트오더"
                             }
    },
    {
        "id":  "2009662047",
        "name":  "코지하우스 노원점",
        "category":  "양식",
        "roadAddress":  "동일로 1419 1층",
        "x":  "127.0601125",
        "y":  "37.6552875",
        "distance":  "600m",
        "hasBooking":  null,
        "hasNPay":  false,
        "visitorReviewCount":  "1,481",
        "imageUrl":  "https://ldb-phinf.pstatic.net/20260219_231/1771466879338Dor9H_JPEG/KakaoTalk_20251229_111436052.jpg",
        "phone":  "02-936-6683",
        "newOpening":  true,
        "newBusinessHours":  {
                                 "status":  "브레이크타임",
                                 "description":  "17:00에 영업 시작"
                             }
    }
]
```

### 카카오맵

#### GET /api/kakao-map/coordinate

좌표 기준 카카오맵 검색

검색어와 중심 좌표(경도, 위도)를 기준으로 카카오맵 검색 결과 원본 데이터를 조회합니다.

##### Sample Request

```http
GET /api/kakao-map/coordinate?query=%EC%B9%B4%ED%8E%98&x=127.06283102249932&y=37.514322572335935&radius=2000&page=1
```

##### Parameters

| Name | In | Required | Type | Description | Example |
| --- | --- | --- | --- | --- | --- |
| query | query | True | string | 검색어 | 카페 |
| x | query | True | string | 중심 좌표 경도(longitude) | 127.06283102249932 |
| y | query | True | string | 중심 좌표 위도(latitude) | 37.514322572335935 |
| radius | query | False | string | 검색 반경(미터) | 2000 |
| page | query | False | string | 결과 페이지 번호 | 1 |

##### Responses

| Status | Description |
| --- | --- |
| 502 | 카카오맵 검색 호출에 실패했습니다 |
| 400 | 요청 파라미터가 올바르지 않습니다 |
| 500 | 카카오맵 REST API 키가 설정되어 있지 않습니다 |
| 200 | 좌표 기반 검색에 성공했습니다 |

##### Example Response (502)

```json
{
    "message":  "카카오맵 검색 호출에 실패했습니다",
    "detail":  "GET 요청 처리 중 외부 API 응답을 정상적으로 받지 못했습니다"
}
```

##### Example Response (500)

```json
{
    "message":  "애플리케이션 설정 오류입니다",
    "detail":  "KAKAO_REST_API_KEY is not configured"
}
```

##### Example Response (200)

```json
{
    "meta":  {
                 "pageable_count":  14,
                 "total_count":  14,
                 "is_end":  true
             },
    "documents":  [
                      {
                          "id":  "26338954",
                          "place_name":  "카카오프렌즈 코엑스점",
                          "distance":  "418",
                          "place_url":  "http://place.map.kakao.com/26338954",
                          "category_name":  "가정,생활 \u003e 문구,사무용품 \u003e 디자인문구 \u003e 카카오프렌즈",
                          "address_name":  "서울 강남구 삼성동 159",
                          "road_address_name":  "서울 강남구 영동대로 513",
                          "phone":  "02-6002-1880",
                          "x":  "127.05902969025047",
                          "y":  "37.51207412593136"
                      }
                  ]
}
```

#### GET /api/kakao-map/search

카카오맵 검색

검색어와 페이지 번호로 카카오맵 검색 결과 원본 데이터를 조회합니다.

##### Sample Request

```http
GET /api/kakao-map/search?q=%EB%85%B8%EC%9B%90%EC%97%AD%20%EB%A7%9B%EC%A7%91&page=1
```

##### Parameters

| Name | In | Required | Type | Description | Example |
| --- | --- | --- | --- | --- | --- |
| q | query | True | string | 검색어 | 노원역 맛집 |
| page | query | False | string | 결과 페이지 번호 | 1 |

##### Responses

| Status | Description |
| --- | --- |
| 502 | 카카오맵 검색 호출에 실패했습니다 |
| 200 | 검색에 성공했습니다 |
| 400 | 요청 파라미터가 올바르지 않습니다 |

##### Example Response (502)

```json
{
    "message":  "카카오맵 검색 호출에 실패했습니다",
    "detail":  "GET 요청 처리 중 외부 API 응답을 정상적으로 받지 못했습니다"
}
```

##### Example Response (200)

```json
[
    {
        "confirmid":  "16590379",
        "name":  "홍대수제버거",
        "address":  "서울 노원구 상계동 323-11",
        "new_address":  "서울 노원구 노해로85길 7",
        "lat":  37.6556909,
        "lon":  127.06482209,
        "last_cate_name":  "햄버거",
        "reviewCount":  241,
        "rating_average":  3.2,
        "img":  "http://t1.daumcdn.net/local/kakaomapPhoto/review/ad20fa578eafba8d3709a3059aba28aa75e3eed0?original"
    },
    {
        "confirmid":  "26086942",
        "name":  "미도참치 노원본점",
        "address":  "서울 노원구 상계동 332-3 1층",
        "new_address":  "서울 노원구 노해로81길 22-22",
        "lat":  37.65589329,
        "lon":  127.06376607,
        "last_cate_name":  "일식,생선회",
        "reviewCount":  263,
        "rating_average":  4.3,
        "img":  "http://t1.daumcdn.net/local/kakaomapPhoto/review/ff425c1be1553b625c301b5134c13be36b1eb390?original"
    }
]
```
