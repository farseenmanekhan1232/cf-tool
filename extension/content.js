// CF-Tool Login Helper
// Automatically sends Codeforces cookies to cf-tool when it detects the cf_port parameter

(function() {
  'use strict';

  // Check if URL has cf_port parameter
  const urlParams = new URLSearchParams(window.location.search);
  const port = urlParams.get('cf_port');

  if (!port) {
    return; // No cf-tool login in progress
  }

  // Check if user is logged in by looking for profile link
  function getHandle() {
    const profileLink = document.querySelector('a[href^="/profile/"]');
    if (profileLink) {
      return profileLink.textContent.trim();
    }
    return null;
  }

  // Wait for page to fully load and check login status
  function checkAndSend() {
    const handle = getHandle();
    
    if (!handle) {
      // Not logged in, show message
      showNotification('Please log in to Codeforces first', 'warning');
      return;
    }

    // Send cookies to cf-tool
    sendCookies(handle, port);
  }

  // Send cookies to local server
  function sendCookies(handle, port) {
    const payload = {
      cookies: document.cookie,
      handle: handle
    };

    fetch(`http://localhost:${port}/callback`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(payload)
    })
    .then(response => {
      if (response.ok) {
        showNotification('✓ Login sent to cf-tool!', 'success');
        // Clean up URL
        const cleanUrl = window.location.origin + window.location.pathname;
        window.history.replaceState({}, document.title, cleanUrl);
      } else {
        showNotification('Failed to send login to cf-tool', 'error');
      }
    })
    .catch(error => {
      showNotification('Error: ' + error.message, 'error');
    });
  }

  // Show notification banner
  function showNotification(message, type) {
    const banner = document.createElement('div');
    banner.style.cssText = `
      position: fixed;
      top: 20px;
      right: 20px;
      padding: 16px 24px;
      border-radius: 8px;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      font-size: 14px;
      font-weight: 500;
      z-index: 999999;
      box-shadow: 0 4px 12px rgba(0,0,0,0.15);
      animation: slideIn 0.3s ease;
    `;

    if (type === 'success') {
      banner.style.background = '#10B981';
      banner.style.color = 'white';
    } else if (type === 'warning') {
      banner.style.background = '#F59E0B';
      banner.style.color = 'white';
    } else {
      banner.style.background = '#EF4444';
      banner.style.color = 'white';
    }

    banner.textContent = message;
    document.body.appendChild(banner);

    // Add animation
    const style = document.createElement('style');
    style.textContent = `
      @keyframes slideIn {
        from { transform: translateX(100%); opacity: 0; }
        to { transform: translateX(0); opacity: 1; }
      }
    `;
    document.head.appendChild(style);

    // Remove after 3 seconds
    setTimeout(() => {
      banner.style.animation = 'slideIn 0.3s ease reverse';
      setTimeout(() => banner.remove(), 300);
    }, 3000);
  }

  // Run when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', checkAndSend);
  } else {
    checkAndSend();
  }
})();
