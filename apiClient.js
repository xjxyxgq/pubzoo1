// API 客户端工具类，统一处理认证和请求
class ApiClient {
  constructor(baseURL = '') {
    this.baseURL = baseURL;
  }

  // 获取认证头
  getAuthHeaders() {
    const token = localStorage.getItem('auth_token');
    console.log('获取token从 localStorage:', token ? token.substring(0, 20) + '...' : 'null'); // 添加调试日志
    return token ? { Authorization: `Bearer ${token}` } : {};
  }

  // 通用请求方法
  async request(endpoint, options = {}) {
    const url = `${this.baseURL}${endpoint}`;
    const authHeaders = this.getAuthHeaders();
    const config = {
      headers: {
        'Content-Type': 'application/json',
        ...authHeaders,
        ...(options.headers || {}),
      },
      ...options,
    };
    
    // 如果是FormData，移除Content-Type让浏览器自动设置
    if (options.body instanceof FormData) {
      delete config.headers['Content-Type'];
    }
    
    console.log('请求配置:', url, config.headers); // 添加调试日志

    try {
      const response = await fetch(url, config);
      
      console.log('响应状态:', response.status); // 添加调试日志
      
      // 如果是 401 未授权，清除本地认证信息
      if (response.status === 401) {
        console.error('401 未授权错误, 清除本地认证信息'); // 添加调试日志
        localStorage.removeItem('auth_token');
        localStorage.removeItem('auth_user');
        window.location.reload(); // 刷新页面，重新进入登录流程
        throw new Error('未授权访问，请重新登录');
      }

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data = await response.json();
      // 返回axios兼容的格式
      return {
        data: data,
        status: response.status,
        statusText: response.statusText,
        headers: response.headers
      };
    } catch (error) {
      console.error('API request failed:', error);
      throw error;
    }
  }

  // GET 请求
  async get(endpoint, params = {}) {
    const searchParams = new URLSearchParams(params);
    const queryString = searchParams.toString();
    const url = queryString ? `${endpoint}?${queryString}` : endpoint;
    
    return this.request(url, { method: 'GET' });
  }

  // POST 请求
  async post(endpoint, data = {}, config = {}) {
    const requestOptions = {
      method: 'POST',
      ...config
    };
    
    // 如果data是FormData，不设置Content-Type，让浏览器自动设置
    if (data instanceof FormData) {
      requestOptions.body = data;
      // 不设置Content-Type，让浏览器自动处理FormData的boundary
    } else {
      requestOptions.body = JSON.stringify(data);
      requestOptions.headers = {
        'Content-Type': 'application/json',
        ...(config.headers || {})
      };
    }
    
    return this.request(endpoint, requestOptions);
  }

  // PUT 请求
  async put(endpoint, data = {}) {
    return this.request(endpoint, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  // DELETE 请求
  async delete(endpoint) {
    return this.request(endpoint, { method: 'DELETE' });
  }

  // OPTIONS 请求
  async options(endpoint) {
    return this.request(endpoint, { method: 'OPTIONS' });
  }

  // 兼容axios风格的get方法 (带params参数)
  async axiosGet(url, config = {}) {
    const { params = {}, ...otherConfig } = config;
    const searchParams = new URLSearchParams(params);
    const queryString = searchParams.toString();
    const endpoint = queryString ? `${url}?${queryString}` : url;
    
    return this.request(endpoint, { 
      method: 'GET',
      ...otherConfig
    });
  }

  // 兼容axios风格的post方法
  async axiosPost(url, data, config = {}) {
    return this.request(url, {
      method: 'POST',
      body: JSON.stringify(data),
      ...config
    });
  }
}

// 创建全局实例
const apiClient = new ApiClient();

export default apiClient;