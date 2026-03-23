import { route } from 'preact-router';
import { useAuth } from '../hooks/useAuth.js';

export default function Header() {
  const { user, logout } = useAuth();

  const handleLogoClick = () => {
    route('/');
  };

  const handleLogout = async () => {
    await logout();
    route('/login');
  };

  return (
    <header class="app-header">
      <div class="header-content">
        <div class="logo" onClick={handleLogoClick}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polygon points="5 3 19 12 5 21 5 3" />
          </svg>
          <span>Tenkile</span>
        </div>

        <nav class="header-nav">
          <button class="nav-link" onClick={() => route('/')}>
            Dashboard
          </button>
        </nav>

        <div class="header-user">
          {user && (
            <span class="username">
              {user.username}
              {user.role === 'admin' && <span class="admin-badge">Admin</span>}
            </span>
          )}
          <button class="btn btn-logout" onClick={handleLogout}>
            Logout
          </button>
        </div>
      </div>
    </header>
  );
}
