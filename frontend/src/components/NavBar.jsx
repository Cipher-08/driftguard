import React from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { ShieldCheck, LogOut } from 'lucide-react';
import { isAuthenticated, getUser, clearSession } from '../lib/auth';

const navLinks = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/drifts', label: 'Drifts' },
  { to: '/resources', label: 'Resources' },
  { to: '/accounts', label: 'Cloud Accounts' },
];

export default function NavBar() {
  const location = useLocation();
  const navigate = useNavigate();
  const authed = isAuthenticated();
  const user = getUser();

  // Hide the nav entirely on auth screens.
  if (location.pathname === '/login' || location.pathname === '/register') {
    return null;
  }

  const logout = () => {
    clearSession();
    navigate('/login');
  };

  return (
    <nav className="bg-white border-b border-gray-200 px-6 py-3 flex gap-6 items-center shadow-sm">
      <Link to="/dashboard" className="flex items-center gap-2 font-bold text-brand-700 text-xl mr-4">
        <ShieldCheck className="w-6 h-6 text-brand-600" />
        DriftGuard
      </Link>
      {authed &&
        navLinks.map((link) => (
          <Link
            key={link.to}
            to={link.to}
            className={`text-gray-600 hover:text-brand-600 transition font-medium ${
              location.pathname === link.to ? 'text-brand-700 border-b-2 border-brand-600 pb-0.5' : ''
            }`}
          >
            {link.label}
          </Link>
        ))}
      <div className="ml-auto flex items-center gap-4">
        {authed ? (
          <>
            {user?.email && <span className="text-sm text-gray-500 hidden sm:inline">{user.email}</span>}
            <button
              onClick={logout}
              className="flex items-center gap-1 text-sm text-gray-600 hover:text-red-600 transition"
            >
              <LogOut className="w-4 h-4" /> Logout
            </button>
          </>
        ) : (
          <Link to="/login" className="text-brand-600 font-medium hover:text-brand-700">
            Login
          </Link>
        )}
      </div>
    </nav>
  );
}
