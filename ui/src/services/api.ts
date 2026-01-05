import axios from 'axios';

const api = axios.create({
  baseURL: '/app/api/v1',
  timeout: 10000,
});

// Response interceptor for error handling
api.interceptors.response.use(
  (response) => response.data,
  (error) => {
    // Handle global errors here if needed
    return Promise.reject(error);
  }
);

export default api;

export interface APIResponse<T = any> {
  code: number;
  message: string;
  data: T;
}

// Feishu Services
export const getVersion = () => api.get<any, APIResponse<{ version: string }>>('/feishu/version');
export const sendCard = (data: any) => api.post<any, APIResponse<any>>('/feishu/api/send-card', data);

// OA Services
export const getJsonAll = () => api.get<any, APIResponse<any[]>>('/oa/get-json-all');
export const getJsonById = (id: string) => api.get<any, APIResponse<any>>(`/oa/get-json/${id}`);
export const getLatestJson = () => api.get<any, APIResponse<any>>('/oa/get-latest-json');
export const storeJson = (data: any) => api.post<any, APIResponse<any>>('/oa/store-json', data);

// Jenkins Services
export const triggerTestFlow = (data: { receive_id: string; receive_id_type: string }) => 
  api.post<any, APIResponse<any>>('/jk/test-flow', data);

export const updateFeishuToken = (data: { user_access_token?: string; user_refresh_token?: string }) =>
  api.post<any, APIResponse<any>>('/jk/feishu/token', data);

// Robot Services
export const getRobots = (params?: { name?: string; page?: number; page_size?: number }) => 
  api.get<any, APIResponse<{ items: any[]; total: number }>>('/robot/query', { params });

export const addRobot = (data: any) => 
  api.post<any, APIResponse<any>>('/robot/addrobot', data);

export const updateRobot = (data: any) => 
  api.post<any, APIResponse<any>>('/robot/updaterobot', data);

export const deleteRobot = (data: { name: string }) => 
  api.post<any, APIResponse<any>>('/robot/delrobot', data);
