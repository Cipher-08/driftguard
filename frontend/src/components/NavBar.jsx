import React from 'react';
import { Link, useLocation } from 'react-router-dom';

const navLinks = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/drifts', label: 'Drifts' },
  { to: '/resources', label: 'Resources' },
  { to: '/add-gcp', label: 'Connect GCP', highlight: true },
  { to: '/login', label: 'Login' },
];

export default function NavBar() {
  const location = useLocation();
  return (
    <nav className="bg-brand-100 px-6 py-3 flex gap-6 items-center shadow">
      <span className="font-bold text-brand-700 text-xl mr-8">DriftGuard</span>
      {navLinks.map(link => (
        <Link
          key={link.to}
          to={link.to}
          className={`text-brand-700 hover:text-brand-600 transition font-medium ${link.highlight ? 'bg-yellow-100 px-3 py-1 rounded shadow-sm border border-yellow-300' : ''} ${location.pathname === link.to ? 'underline' : ''}`}
        >
          {link.label}
        </Link>
      ))}
    </nav>
  );
}
