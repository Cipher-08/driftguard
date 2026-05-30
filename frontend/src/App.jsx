import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';


import NavBar from './components/NavBar';
import DriftList from './pages/DriftList';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Resources from './pages/Resources';
import AddGcpCredential from './pages/AddGcpCredential';

export default function App() {
  return (
    <Router>
      <NavBar />
      <div className="pt-6">
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/drifts" element={<DriftList />} />
          <Route path="/resources" element={<Resources />} />
          <Route path="/add-gcp" element={<AddGcpCredential />} />
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </div>
    </Router>
  );
}
