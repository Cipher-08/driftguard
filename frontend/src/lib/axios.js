import axios from 'axios';
import { getToken, clearSession } from './auth';

const instance = axios.create();

// Attach JWT to every request if present.
instance.interceptors.request.use((config) => {
  const token = getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// On 401, clear the session and bounce to login.
instance.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401 && !window.location.pathname.startsWith('/login')) {
      clearSession();
      window.location.assign('/login');
    }
    return Promise.reject(err);
  }
);

export default instance;
