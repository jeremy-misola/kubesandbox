import { useAuth } from '../auth';

export function UserInfo() {
  const { user, isAuthenticated, logout } = useAuth();

  if (!isAuthenticated || !user) {
    return null;
  }

  return (
    <div className="user-info">
      <div className="user-info__icon">
        <span className="material-symbols-outlined">account_circle</span>
      </div>
      <div className="user-info__details">
        <strong className="user-info__name">{user.name ?? user.email}</strong>
        {user.email && user.name && (
          <p className="user-info__email">{user.email}</p>
        )}
      </div>
      <button 
        className="user-info__logout" 
        onClick={logout}
        title="Sign out"
      >
        <span className="material-symbols-outlined">logout</span>
      </button>
    </div>
  );
}