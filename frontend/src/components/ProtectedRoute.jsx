import React from 'react';
import { Navigate } from 'react-router-dom';
import { isAuthenticated } from '../lib/auth';

// Wraps routes that require a logged-in user.
export default function ProtectedRoute({ children }) {
  if (!isAuthenticated()) {
    return <Navigate to="/login" replace />;
  }
  return children;
}
