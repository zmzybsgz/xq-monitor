package xueqiu

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"xueqiu-monitor/internal/model"
)

// Client 雪球 API 客户端
type Client struct {
	Cookie     string
	HTTPClient *http.Client
	mu         sync.Mutex
}

// NewClient 创建雪球客户端，cookie 为空时会尝试匿名访问
func NewClient(cookie string) *Client {
	return &Client{
		Cookie:     cookie,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// currentResponse cubes/rebalancing/current.json 响应
type currentResponse struct {
	LastRb struct {
		Cash     float64 `json:"cash"`
		Holdings []struct {
			StockSymbol string  `json:"stock_symbol"`
			StockName   string  `json:"stock_name"`
			Weight      float64 `json:"weight"`
		} `json:"holdings"`
	} `json:"last_rb"`
	ErrorCode        interface{} `json:"error_code"`
	ErrorDescription string      `json:"error_description"`
}

// quoteResponse v5/stock/realtime/quotec.json 响应
type quoteResponse struct {
	Data []struct {
		Symbol  string  `json:"symbol"`
		Current float64 `json:"current"`
	} `json:"data"`
}

// GetHoldings 获取组合当前完整持仓
func (c *Client) GetHoldings(portfolioID string) ([]model.Holding, error) {
	cookie := c.resolveCookie()

	apiURL := "https://xueqiu.com/cubes/rebalancing/current.json"
	params := url.Values{}
	params.Set("cube_symbol", portfolioID)

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	setCommonHeaders(req, cookie)

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	return parseCurrentResponse(body)
}

// parseCurrentResponse 解析持仓响应 JSON，空持仓返回 nil 由调用方判断
func parseCurrentResponse(body []byte) ([]model.Holding, error) {
	var result currentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	switch v := result.ErrorCode.(type) {
	case string:
		if v != "" && v != "0" {
			return nil, fmt.Errorf("雪球返回错误 %s: %s", v, result.ErrorDescription)
		}
	case float64:
		if v != 0 {
			return nil, fmt.Errorf("雪球返回错误 %.0f: %s", v, result.ErrorDescription)
		}
	}

	if len(result.LastRb.Holdings) == 0 {
		return nil, nil
	}

	holdings := make([]model.Holding, 0, len(result.LastRb.Holdings))
	for _, s := range result.LastRb.Holdings {
		holdings = append(holdings, model.Holding{
			Symbol: s.StockSymbol,
			Name:   s.StockName,
			Weight: math.Round(s.Weight*100) / 100,
		})
	}

	sort.Slice(holdings, func(i, j int) bool {
		return holdings[i].Symbol < holdings[j].Symbol
	})

	return holdings, nil
}

// GetStockPrices 批量获取股票当前价格
func (c *Client) GetStockPrices(symbols []string) (map[string]float64, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	apiURL := "https://stock.xueqiu.com/v5/stock/realtime/quotec.json?symbol=" + strings.Join(symbols, ",")
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, c.Cookie)

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("获取行情失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseQuoteResponse(body)
}

// parseQuoteResponse 解析行情响应 JSON
func parseQuoteResponse(body []byte) (map[string]float64, error) {
	var result quoteResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析行情 JSON 失败: %w", err)
	}

	prices := make(map[string]float64, len(result.Data))
	for _, d := range result.Data {
		prices[d.Symbol] = d.Current
	}
	return prices, nil
}

// resolveCookie 如果未配置 cookie，访问首页获取匿名 session 并缓存，加锁保证并发安全
func (c *Client) resolveCookie() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Cookie != "" {
		return c.Cookie
	}

	req, _ := http.NewRequest("GET", "https://xueqiu.com", nil)
	setCommonHeaders(req, "")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var parts []string
	for _, ck := range resp.Cookies() {
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	time.Sleep(time.Second)
	c.Cookie = strings.Join(parts, "; ")
	return c.Cookie
}

// doWithRetry 对幂等 GET 请求做最多 3 次重试，指数退避
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	const attempts = 3
	var lastErr error
	for i := 0; i < attempts; i++ {
		// 每次重试克隆请求，避免复用同一 *http.Request 导致连接池异常
		r := req.Clone(req.Context())
		resp, err := c.HTTPClient.Do(r)
		if err == nil && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		}
		if resp != nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		if i < attempts-1 {
			time.Sleep(time.Duration(1<<i) * time.Second)
		}
	}
	return nil, fmt.Errorf("重试 %d 次后失败: %w", attempts, lastErr)
}

func setCommonHeaders(req *http.Request, cookie string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://xueqiu.com/")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
