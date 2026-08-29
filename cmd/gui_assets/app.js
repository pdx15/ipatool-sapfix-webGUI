/**
 * ipatool GUI — Interactive Web Application Logic
 * Supports Russian (default) and English localization
 */

// Application State
const state = {
  lang: localStorage.getItem('ipatool_lang') || 'ru',
  theme: localStorage.getItem('ipatool_theme') || 'dark',
  activeTab: 'search',
  account: null,
  isAuthenticated: false,
  activeDownloads: new Map(), // jobId -> job object
  downloadHistory: JSON.parse(localStorage.getItem('ipatool_download_history') || '[]'),
  lastPendingLogin: null, // { email, password } for 2FA retry
  currentJobId: null,
  lastVersionsName: '' // app name remembered when opening version history from a search card
};

// I18N Dictionaries
const i18n = {
  ru: {
    app_subtitle: 'App Store IPA Downloader для Windows',
    auth_checking: 'Проверка аккаунта...',
    auth_not_logged_in: 'Не авторизован',
    auth_logged_in: 'Авторизован',
    tab_search: 'Поиск приложений',
    tab_direct: 'Прямая загрузка',
    tab_versions: 'История версий',
    tab_downloads: 'Загрузки',
    tab_account: 'Аккаунт Apple ID',
    tab_guide: 'Инструкция и FAQ',
    search_title: 'Поиск приложений в App Store',
    search_desc: 'Введите название приложения или вставьте ссылку на App Store',
    platform_label: 'Платформа:',
    limit_label: 'Результатов:',
    search_button: 'Найти',
    popular_searches: 'Популярное:',
    searching_status: 'Поиск приложений в Apple App Store...',
    search_prompt_title: 'Введите название для поиска',
    search_prompt_desc: 'Найдите нужное приложение, получите для него лицензию и скачайте зашифрованный установочный пакет .IPA',
    no_results_title: 'Ничего не найдено',
    no_results_desc: 'Попробуйте изменить поисковый запрос или переключить платформу (iPhone / iPad).',
    direct_title: 'Прямая загрузка приложения',
    direct_desc: 'Загрузите приложение по Bundle ID, App ID или прямой ссылке на App Store',
    direct_id_label: 'Идентификатор приложения (Bundle ID, App ID или Ссылка):',
    direct_id_hint: 'Вы можете указать Bundle Identifier (например com.example.app), цифровой Track ID (например 686449807) или ссылку на страницу в App Store.',
    version_id_label: 'ID версии (необязательно):',
    output_folder_label: 'Папка для сохранения:',
    open_folder_btn: 'Открыть',
    auto_purchase_title: 'Автоматически получить лицензию (Purchase)',
    auto_purchase_desc: 'Необходимо, если приложение ещё ни разу не скачивалось с этого Apple ID (только для бесплатных)',
    start_download_btn: 'Начать скачивание .IPA',
    versions_title: 'История версий приложения',
    versions_desc: 'Просмотр всех предыдущих сборок и скачивание старых версий приложения',
    fetch_versions_btn: 'Получить версии',
    fetching_versions_status: 'Запрос списка сборок с серверов Apple...',
    version_col_build: 'ID сборки (Version ID)',
    version_col_display: 'Версия (Display)',
    version_col_date: 'Дата выхода',
    version_col_action: 'Действие',
    active_downloads_title: 'Активные загрузки',
    active_downloads_desc: 'Текущие процессы скачивания и обработки пакетов',
    open_downloads_folder: 'Открыть папку загрузок',
    no_active_downloads: 'Нет активных загрузок',
    download_history_title: 'История завершенных загрузок',
    download_history_desc: 'Скачанные файлы .IPA на вашем компьютере',
    refresh_btn: 'Обновить',
    no_history_downloads: 'История пока пуста',
    account_card_title: 'Текущий Apple ID',
    account_card_desc: 'Состояние авторизации и сведения об активной сессии',
    badge_authenticated: 'Авторизован',
    info_country: 'Регион App Store:',
    info_dsid: 'DSID:',
    info_keychain: 'Хранилище сессии:',
    info_keychain_ok: 'Системное хранилище Windows',
    export_session_btn: 'Экспорт сессии (JSON)',
    logout_btn: 'Выйти из аккаунта',
    login_prompt_title: 'Вход в Apple ID',
    login_prompt_desc: 'Авторизация необходима для получения лицензий и зашифрованных установочных файлов из App Store.',
    login_email_label: 'Apple ID (Email):',
    login_password_label: 'Пароль Apple ID:',
    login_security_hint: 'Пароль передается напрямую на защищенные серверы Apple (SRP-6a/GSA) и не сохраняется третьими лицами.',
    login_btn: 'Войти в Apple ID',
    session_mgmt_title: 'Сессии и Перенос',
    session_mgmt_desc: 'Импортируйте сессию, созданную на другом устройстве, чтобы не вводить пароль и 2FA повторно',
    import_session_title: '📥 Импорт файла сессии',
    import_session_desc: 'Выберите ранее сохраненный файл account-session.json:',
    dropzone_text: 'Нажмите, чтобы выбрать .json файл сессии',
    paste_json_label: 'Или вставьте JSON сессии напрямую:',
    import_text_btn: 'Импортировать из текста',
    anisette_card_title: '⚙️ Проверка iCloud для Windows',
    anisette_card_desc: 'Проверьте, установлена ли нужная версия iCloud на этом компьютере',
    anisette_alert_title: 'Для Windows-версии ipatool:',
    anisette_alert_desc: 'Проверка установленного iCloud…',
    icloud_installed: '✅ Классическая iCloud для Windows установлена.',
    icloud_installed_classic: 'Найдена папка Apple\\Internet Services с AOSKit.dll.',
    icloud_not_installed: '❌ Классическая iCloud для Windows не найдена. Скачайте и установите её по ссылке ниже, затем войдите в неё своим Apple ID.',
    icloud_download_btn: 'Скачать классическую iCloud для Windows',
    icloud_store_url: 'https://updates.cdn-apple.com/2020/windows/001-39935-20200911-1A70AA56-F448-11EA-8CC0-99D41950005E/iCloudSetup.exe',
    guide_title: '📱 Инструкция: Как установить .IPA на iPhone или iPad',
    guide_desc: 'Скачанный файл .IPA является официальным пакетом App Store. Вот лучшие способы установить его на ваше устройство:',
    faq_title: 'Часто задаваемые вопросы (FAQ)',
    modal_2fa_title: 'Двухфакторная аутентификация',
    modal_2fa_desc: 'Введите 6-значный проверочный код, отправленный на ваши устройства Apple или в SMS',
    cancel_btn: 'Отмена',
    verify_btn: 'Подтвердить',
    close_btn: 'Закрыть',
    open_in_explorer_btn: 'Показать в проводнике',
    step_license: 'Проверка лицензии / Покупка',
    step_download: 'Загрузка пакета .IPA из App Store',
    step_sinf: 'Применение цифровой подписи (sinf)',
    step_complete: 'Готово к установке',
    free_price: 'Бесплатно',
    download_btn: 'Скачать IPA',
    license_btn: 'Лицензия',
    versions_btn: 'Версии',
    copied_toast: 'Скопировано в буфер обмена'
  },
  en: {
    app_subtitle: 'App Store IPA Downloader for Windows',
    auth_checking: 'Checking account...',
    auth_not_logged_in: 'Not Logged In',
    auth_logged_in: 'Authenticated',
    tab_search: 'Search Apps',
    tab_direct: 'Direct Download',
    tab_versions: 'Version History',
    tab_downloads: 'Downloads',
    tab_account: 'Apple ID Account',
    tab_guide: 'Guide & FAQ',
    search_title: 'Search Apple App Store',
    search_desc: 'Enter an app title or paste an App Store URL',
    platform_label: 'Platform:',
    limit_label: 'Results:',
    search_button: 'Search',
    popular_searches: 'Popular:',
    searching_status: 'Searching apps in Apple App Store...',
    search_prompt_title: 'Search for Apps',
    search_prompt_desc: 'Find any iOS/iPadOS/tvOS app, acquire free licenses and download encrypted .IPA packages',
    no_results_title: 'No Results Found',
    no_results_desc: 'Try refining your search keyword or change platform (iPhone / iPad).',
    direct_title: 'Direct App Download',
    direct_desc: 'Download an app by Bundle ID, App ID or direct App Store URL',
    direct_id_label: 'App Identifier (Bundle ID, App ID or URL):',
    direct_id_hint: 'You can provide a Bundle Identifier (e.g. com.example.app), numeric Track ID (e.g. 686449807), or an App Store web link.',
    version_id_label: 'Version ID (optional):',
    output_folder_label: 'Output Folder:',
    open_folder_btn: 'Open',
    auto_purchase_title: 'Auto-acquire license (Purchase)',
    auto_purchase_desc: 'Required if this app was never downloaded with this Apple ID before (free apps only)',
    start_download_btn: 'Start .IPA Download',
    versions_title: 'App Version History',
    versions_desc: 'View all historical builds and download previous app versions',
    fetch_versions_btn: 'Fetch Versions',
    fetching_versions_status: 'Fetching build list from Apple servers...',
    version_col_build: 'Build ID (Version ID)',
    version_col_display: 'Display Version',
    version_col_date: 'Release Date',
    version_col_action: 'Action',
    active_downloads_title: 'Active Downloads',
    active_downloads_desc: 'Current download and package signing processes',
    open_downloads_folder: 'Open Downloads Folder',
    no_active_downloads: 'No active downloads',
    download_history_title: 'Download History',
    download_history_desc: 'Completed .IPA packages on your computer',
    refresh_btn: 'Refresh',
    no_history_downloads: 'History is empty',
    account_card_title: 'Current Apple ID',
    account_card_desc: 'Authentication state and active session details',
    badge_authenticated: 'Authenticated',
    info_country: 'App Store Storefront:',
    info_dsid: 'DSID:',
    info_keychain: 'Session Storage:',
    info_keychain_ok: 'Windows Credential Storage',
    export_session_btn: 'Export Session (JSON)',
    logout_btn: 'Log Out (Revoke)',
    login_prompt_title: 'Sign in with Apple ID',
    login_prompt_desc: 'Authentication is required to obtain license signatures and encrypted packages from the App Store.',
    login_email_label: 'Apple ID (Email):',
    login_password_label: 'Apple ID Password:',
    login_security_hint: 'Your password is sent directly to Apple secure servers (SRP-6a/GSA) over HTTPS and is never stored unencrypted.',
    login_btn: 'Sign In',
    session_mgmt_title: 'Sessions & Portability',
    session_mgmt_desc: 'Import a session generated on another machine to skip typing passwords and 2FA codes',
    import_session_title: '📥 Import Session File',
    import_session_desc: 'Choose a previously saved account-session.json file:',
    dropzone_text: 'Click or drop .json session file here',
    paste_json_label: 'Or paste session JSON text directly:',
    import_text_btn: 'Import from text',
    anisette_card_title: '⚙️ iCloud Check for Windows',
    anisette_card_desc: 'Verify that the required iCloud version is installed on this computer',
    anisette_alert_title: 'For Windows users of ipatool:',
    anisette_alert_desc: 'Checking installed iCloud…',
    icloud_installed: '✅ Classic iCloud for Windows is installed.',
    icloud_installed_classic: 'Found Apple\\Internet Services with AOSKit.dll.',
    icloud_not_installed: '❌ Classic iCloud for Windows was not found. Download and install it via the link below, then sign in with your Apple ID.',
    icloud_download_btn: 'Download classic iCloud for Windows',
    icloud_store_url: 'https://updates.cdn-apple.com/2020/windows/001-39935-20200911-1A70AA56-F448-11EA-8CC0-99D41950005E/iCloudSetup.exe',
    guide_title: '📱 Guide: How to install .IPA on iPhone or iPad',
    guide_desc: 'Downloaded .IPA files are genuine App Store packages. Here are the best methods to install them:',
    faq_title: 'Frequently Asked Questions (FAQ)',
    modal_2fa_title: 'Two-Factor Authentication',
    modal_2fa_desc: 'Enter the 6-digit verification code sent to your Apple devices or SMS',
    cancel_btn: 'Cancel',
    verify_btn: 'Verify Code',
    close_btn: 'Close',
    open_in_explorer_btn: 'Show in Explorer',
    step_license: 'License Verification / Purchase',
    step_download: 'Downloading .IPA package from App Store',
    step_sinf: 'Applying digital signature (sinf replication)',
    step_complete: 'Ready for installation',
    free_price: 'Free',
    download_btn: 'Download IPA',
    license_btn: 'License',
    versions_btn: 'Versions',
    copied_toast: 'Copied to clipboard'
  }
};

// Apply UI Localization
function applyLanguage() {
  const dict = i18n[state.lang] || i18n.ru;
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    if (dict[key]) {
      el.textContent = dict[key];
    }
  });

  const langLabel = document.getElementById('lang-label');
  if (langLabel) langLabel.textContent = state.lang.toUpperCase();

  // Update account status pill
  updateAccountStatusPill();

  // Re-run iCloud presence check so its text follows the selected language.
  checkICloudStatus();
}

function toggleLanguage() {
  state.lang = state.lang === 'ru' ? 'en' : 'ru';
  localStorage.setItem('ipatool_lang', state.lang);
  applyLanguage();
}

// Theme handling
function applyTheme() {
  document.documentElement.setAttribute('data-theme', state.theme);
  const darkIcon = document.getElementById('theme-icon-dark');
  const lightIcon = document.getElementById('theme-icon-light');
  if (darkIcon && lightIcon) {
    if (state.theme === 'dark') {
      darkIcon.style.display = 'block';
      lightIcon.style.display = 'none';
    } else {
      darkIcon.style.display = 'none';
      lightIcon.style.display = 'block';
    }
  }
}

function toggleTheme() {
  state.theme = state.theme === 'dark' ? 'light' : 'dark';
  localStorage.setItem('ipatool_theme', state.theme);
  applyTheme();
}

// Tab Switching
function switchTab(tabName) {
  state.activeTab = tabName;
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.classList.toggle('active', btn.getAttribute('data-tab') === tabName);
  });
  document.querySelectorAll('.tab-content').forEach(sec => {
    sec.classList.toggle('active', sec.id === `tab-${tabName}`);
  });

  if (tabName === 'downloads') {
    renderDownloadsTab();
  }
}

// Toast Notifications
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  
  let icon = 'ℹ️';
  if (type === 'success') icon = '✅';
  if (type === 'error') icon = '❌';

  toast.innerHTML = `<span>${icon}</span><span>${message}</span>`;
  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    setTimeout(() => toast.remove(), 250);
  }, 4000);
}

// Helper: Copy to Clipboard
function copyToClipboard(text, msg) {
  navigator.clipboard.writeText(text).then(() => {
    showToast(msg || i18n[state.lang].copied_toast, 'success');
  }).catch(() => {
    showToast('Failed to copy', 'error');
  });
}

// Helper: Parse App Store URL
function parseAppStoreUrl(input) {
  if (!input) return input;
  const match = input.match(/id(\d+)/i);
  if (match && match[1]) {
    return match[1];
  }
  return input.trim();
}

// Helper: Format bytes
function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Helper: Toggle Password Visibility
function togglePasswordVisibility(inputId) {
  const input = document.getElementById(inputId);
  if (input) {
    input.type = input.type === 'password' ? 'text' : 'password';
  }
}

// ==========================================
// API Interaction & Account Management
// ==========================================

async function checkICloudStatus() {
  const textEl = document.getElementById('icloud-status-text');
  const iconEl = document.getElementById('icloud-status-icon');
  const linkEl = document.getElementById('icloud-download-link');
  if (!textEl) return;
  const dict = i18n[state.lang] || i18n.ru;

  try {
    const res = await fetch('/api/icloud/status');
    const data = await res.json();

    if (data.installed) {
      if (iconEl) iconEl.textContent = '✅';
      if (data.variant === 'classic') {
        textEl.textContent = `${dict.icloud_installed} ${dict.icloud_installed_classic}`;
      } else {
        textEl.textContent = dict.icloud_installed;
      }
      if (linkEl) linkEl.style.display = 'none';
    } else {
      if (iconEl) iconEl.textContent = '❌';
      textEl.textContent = dict.icloud_not_installed;
      if (linkEl) {
        linkEl.href = data.downloadUrl || dict.icloud_store_url || 'https://updates.cdn-apple.com/2020/windows/001-39935-20200911-1A70AA56-F448-11EA-8CC0-99D41950005E/iCloudSetup.exe';
        linkEl.style.display = 'inline-flex';
      }
    }
  } catch (err) {
    textEl.textContent = dict.anisette_alert_desc || '—';
  }
}

async function fetchStatus() {
  try {
    const res = await fetch('/api/status');
    const data = await res.json();
    
    state.isAuthenticated = data.authenticated;
    state.account = data.account || null;

    updateAccountUI();
  } catch (err) {
    console.error('Failed to fetch status:', err);
    state.isAuthenticated = false;
    state.account = null;
    updateAccountUI();
  }
}

function updateAccountStatusPill() {
  const pill = document.getElementById('account-status-pill');
  const text = document.getElementById('account-status-text');
  if (!pill || !text) return;

  const dict = i18n[state.lang] || i18n.ru;

  if (state.isAuthenticated && state.account) {
    pill.className = 'account-pill logged-in';
    const name = state.account.name || state.account.email.split('@')[0];
    text.textContent = `🟢 ${name} (${state.account.email})`;
  } else {
    pill.className = 'account-pill logged-out';
    text.textContent = `🔴 ${dict.auth_not_logged_in}`;
  }
}

function updateAccountUI() {
  updateAccountStatusPill();

  const loggedInBox = document.getElementById('account-logged-in-box');
  const loggedOutBox = document.getElementById('account-logged-out-box');

  if (state.isAuthenticated && state.account) {
    if (loggedInBox) loggedInBox.style.display = 'block';
    if (loggedOutBox) loggedOutBox.style.display = 'none';

    const nameEl = document.getElementById('account-user-name');
    const emailEl = document.getElementById('account-user-email');
    const countryEl = document.getElementById('account-country');
    const dsidEl = document.getElementById('account-dsid');
    const initialsEl = document.getElementById('account-initials');

    if (nameEl) nameEl.textContent = state.account.name || 'Пользователь Apple';
    if (emailEl) emailEl.textContent = state.account.email;
    if (countryEl) countryEl.textContent = state.account.storefront || 'Apple App Store';
    if (dsidEl) dsidEl.textContent = state.account.dsid || '—';
    if (initialsEl) {
      const char = (state.account.name ? state.account.name[0] : state.account.email[0]).toUpperCase();
      initialsEl.textContent = char;
    }
  } else {
    if (loggedInBox) loggedInBox.style.display = 'none';
    if (loggedOutBox) loggedOutBox.style.display = 'block';
  }
}

// Handle Login
async function handleLogin(e) {
  e.preventDefault();
  const email = document.getElementById('login-email').value.trim();
  const password = document.getElementById('login-password').value;
  const submitBtn = document.getElementById('login-submit-btn');

  if (!email || !password) {
    showToast('Заполните Email и Пароль', 'error');
    return;
  }

  submitBtn.disabled = true;
  submitBtn.textContent = 'Авторизация...';

  try {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password })
    });
    const data = await res.json();

    if (data.authCodeRequired) {
      // 2FA required! Open 2FA modal
      state.lastPendingLogin = { email, password };
      open2FAModal();
      showToast('Требуется код двухфакторной аутентификации', 'info');
    } else if (data.anisetteRequired) {
      // Windows GSA login needs a locally installed & signed-in iCloud to
      // produce anisette headers. Show the precise reason returned by the
      // backend so the user knows exactly which check failed.
      showToast('Ошибка iCloud (anisette): ' + (data.message || 'проверьте установку iCloud'), 'error');
      switchTab('account');
    } else if (data.success) {
      state.isAuthenticated = true;
      state.account = data.account;
      updateAccountUI();
      showToast(`Успешный вход: ${data.account.email}`, 'success');
    } else {
      showToast(data.message || 'Ошибка авторизации Apple ID', 'error');
    }
  } catch (err) {
    showToast('Ошибка сетевого соединения с локальным сервером', 'error');
  } finally {
    submitBtn.disabled = false;
    applyLanguage();
  }
}

// 2FA Modal Handlers
function open2FAModal() {
  const modal = document.getElementById('two-factor-modal');
  const input = document.getElementById('two-factor-code');
  if (modal) {
    modal.style.display = 'flex';
    if (input) {
      input.value = '';
      input.focus();
    }
  }
}

function close2FAModal() {
  const modal = document.getElementById('two-factor-modal');
  if (modal) modal.style.display = 'none';
  state.lastPendingLogin = null;
}

async function handle2FASubmit(e) {
  e.preventDefault();
  const code = document.getElementById('two-factor-code').value.trim();
  const submitBtn = document.getElementById('two-factor-submit-btn');

  if (!code || code.length < 6) {
    showToast('Введите 6-значный код', 'error');
    return;
  }

  if (!state.lastPendingLogin) {
    close2FAModal();
    return;
  }

  submitBtn.disabled = true;
  submitBtn.textContent = 'Проверка...';

  try {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: state.lastPendingLogin.email,
        password: state.lastPendingLogin.password,
        authCode: code
      })
    });
    const data = await res.json();

    if (data.success) {
      close2FAModal();
      state.isAuthenticated = true;
      state.account = data.account;
      updateAccountUI();
      showToast(`Успешный вход: ${data.account.email}`, 'success');
    } else {
      showToast(data.message || 'Неверный 2FA код', 'error');
    }
  } catch (err) {
    showToast('Ошибка проверки кода', 'error');
  } finally {
    submitBtn.disabled = false;
    applyLanguage();
  }
}

// Handle Logout / Revoke
async function confirmLogout() {
  if (!confirm('Вы действительно хотите выйти из учетной записи Apple ID?')) return;

  try {
    const res = await fetch('/api/auth/revoke', { method: 'POST' });
    const data = await res.json();
    if (data.success) {
      state.isAuthenticated = false;
      state.account = null;
      updateAccountUI();
      showToast('Вы успешно вышли из аккаунта', 'success');
    } else {
      showToast(data.message || 'Ошибка выхода', 'error');
    }
  } catch (err) {
    showToast('Ошибка запроса выхода', 'error');
  }
}

// Handle Export Session
async function exportSession() {
  try {
    const res = await fetch('/api/auth/export');
    if (!res.ok) {
      const err = await res.json();
      throw new Error(err.message || 'Не удалось экспортировать');
    }
    const blob = await res.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `account-session-${state.account ? state.account.email : 'apple'}.json`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    showToast('Файл сессии успешно скачан', 'success');
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// Handle Import Session from File
async function handleSessionFileUpload(e) {
  const file = e.target.files[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = async (event) => {
    try {
      const json = event.target.result;
      await submitImportSession(json);
    } catch (err) {
      showToast('Неверный формат JSON файла', 'error');
    }
  };
  reader.readAsText(file);
}

// Handle Import Session from Text
async function importSessionFromText() {
  const text = document.getElementById('session-json-textarea').value.trim();
  if (!text) {
    showToast('Вставьте JSON сессии в текстовое поле', 'error');
    return;
  }
  await submitImportSession(text);
}

async function submitImportSession(jsonString) {
  try {
    const parsed = JSON.parse(jsonString);
    const res = await fetch('/api/auth/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(parsed)
    });
    const data = await res.json();

    if (data.success) {
      showToast('Сессия успешно импортирована!', 'success');
      await fetchStatus();
    } else {
      showToast(data.message || 'Ошибка импорта сессии', 'error');
    }
  } catch (err) {
    showToast('Ошибка структуры JSON сессии', 'error');
  }
}

// ==========================================
// App Search & Browse
// ==========================================

function quickSearch(term) {
  const input = document.getElementById('search-input');
  if (input) {
    input.value = term;
    handleSearch(new Event('submit'));
  }
}

function clearSearch() {
  const input = document.getElementById('search-input');
  const clearBtn = document.getElementById('search-clear-btn');
  if (input) {
    input.value = '';
    input.focus();
  }
  if (clearBtn) clearBtn.style.display = 'none';
  document.getElementById('search-results-wrapper').style.display = 'none';
  document.getElementById('search-empty').style.display = 'block';
  document.getElementById('search-no-results').style.display = 'none';
}

async function handleSearch(e) {
  if (e) e.preventDefault();
  let term = document.getElementById('search-input').value.trim();
  const platform = document.getElementById('search-platform').value;
  const limit = document.getElementById('search-limit').value;

  if (!term) {
    showToast('Введите поисковый запрос', 'info');
    return;
  }

  // Check if it's an App Store URL
  term = parseAppStoreUrl(term);

  const loadingEl = document.getElementById('search-loading');
  const emptyEl = document.getElementById('search-empty');
  const noResultsEl = document.getElementById('search-no-results');
  const resultsWrapper = document.getElementById('search-results-wrapper');
  const resultsGrid = document.getElementById('search-results-grid');
  const resultsCount = document.getElementById('results-count');
  const clearBtn = document.getElementById('search-clear-btn');

  if (clearBtn) clearBtn.style.display = 'block';
  emptyEl.style.display = 'none';
  noResultsEl.style.display = 'none';
  resultsWrapper.style.display = 'none';
  loadingEl.style.display = 'block';

  try {
    const res = await fetch(`/api/search?term=${encodeURIComponent(term)}&platform=${platform}&limit=${limit}`);
    const data = await res.json();

    loadingEl.style.display = 'none';

    if (!data.success) {
      showToast(data.message || 'Ошибка поиска в App Store', 'error');
      emptyEl.style.display = 'block';
      return;
    }

    const apps = data.results || [];
    if (apps.length === 0) {
      noResultsEl.style.display = 'block';
      return;
    }

    resultsCount.textContent = `Найдено приложений: ${apps.length}`;
    resultsGrid.innerHTML = '';

    apps.forEach(app => {
      const card = createAppCard(app, platform);
      resultsGrid.appendChild(card);
    });

    resultsWrapper.style.display = 'block';
  } catch (err) {
    loadingEl.style.display = 'none';
    showToast('Ошибка связи с сервером', 'error');
    emptyEl.style.display = 'block';
  }
}

function createAppCard(app, platform) {
  const card = document.createElement('div');
  card.className = 'app-card';

  const iconUrl = app.artworkUrl512 || app.artworkUrl100 || app.artworkUrl60;
  const priceText = app.price === 0 || !app.price ? (i18n[state.lang]?.free_price || 'Бесплатно') : (app.formattedPrice || `$${app.price}`);
  const dict = i18n[state.lang] || i18n.ru;

  card.innerHTML = `
    <div class="app-main-info">
      ${iconUrl ? `<img src="${iconUrl}" alt="${app.trackName}" class="app-icon-img" loading="lazy" onerror="this.outerHTML='<div class=\\'app-icon-fallback\\'>📱</div>'">` : `<div class="app-icon-fallback">📱</div>`}
      <div class="app-meta">
        <h3 class="app-name" title="${app.trackName}">${app.trackName || 'Без названия'}</h3>
        <p class="app-artist" title="${app.artistName || app.sellerName || ''}">${app.artistName || app.sellerName || 'Apple Developer'}</p>
        <div class="app-badges">
          <span class="badge badge-price">${priceText}</span>
          ${app.version ? `<span class="badge badge-version">v${app.version}</span>` : ''}
          ${app.fileSizeBytes ? `<span class="badge">${formatBytes(app.fileSizeBytes)}</span>` : ''}
          <span class="badge">${platform.toUpperCase()}</span>
        </div>
      </div>
    </div>

    <div class="app-identifiers">
      <div class="id-row">
        <span class="id-label">Bundle ID:</span>
        <span class="id-value">
          <code>${app.bundleId}</code>
          <button type="button" class="copy-icon-btn" onclick="copyToClipboard('${app.bundleId}')" title="Скопировать">📋</button>
        </span>
      </div>
      <div class="id-row">
        <span class="id-label">App ID:</span>
        <span class="id-value">
          <code>${app.trackId}</code>
          <button type="button" class="copy-icon-btn" onclick="copyToClipboard('${app.trackId}')" title="Скопировать">📋</button>
        </span>
      </div>
    </div>

    <div class="app-actions">
      <button class="btn btn-primary" onclick="startAppDownload('${app.bundleId}', ${app.trackId}, '${platform}', '${app.trackName.replace(/'/g, "\\'")}', '${iconUrl || ''}')">
        ⬇️ ${dict.download_btn}
      </button>
      <button class="btn btn-outline" onclick="purchaseApp('${app.bundleId}')" title="Получить лицензию">
        🛒 ${dict.license_btn}
      </button>
      <button class="btn btn-outline" onclick="viewAppVersions('${app.bundleId}', ${app.trackId}, '${app.trackName.replace(/'/g, "\\'")}')" title="История версий">
        🕒 ${dict.versions_btn}
      </button>
    </div>
  `;

  return card;
}

// Purchase App License
async function purchaseApp(bundleId) {
  if (!state.isAuthenticated) {
    showToast('Сначала необходимо войти в Apple ID во вкладке "Аккаунт"', 'error');
    switchTab('account');
    return;
  }

  showToast(`Запрос лицензии для ${bundleId}...`, 'info');

  try {
    const res = await fetch('/api/purchase', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ bundleId })
    });
    const data = await res.json();

    if (data.success) {
      if (data.alreadyOwned) {
        showToast('Лицензия уже имеется на вашем Apple ID', 'success');
      } else {
        showToast('Лицензия успешно получена!', 'success');
      }
    } else {
      showToast(data.message || 'Ошибка приобретения лицензии', 'error');
    }
  } catch (err) {
    showToast('Ошибка запроса к серверу', 'error');
  }
}

// ==========================================
// Download Execution & Real-Time Progress
// ==========================================

async function startAppDownload(bundleId, appId, platform, appName, iconUrl, versionId = '', outputPath = '') {
  if (!state.isAuthenticated) {
    showToast('Для скачивания необходимо войти в Apple ID', 'error');
    switchTab('account');
    return;
  }

  // Open download progress modal
  openDownloadModal(appName, bundleId, iconUrl);

  try {
    const res = await fetch('/api/download', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        bundleId: bundleId || '',
        appId: appId ? parseInt(appId, 10) : 0,
        platform: platform || 'iphone',
        externalVersionId: versionId || '',
        outputPath: outputPath || '',
        purchase: true // auto purchase enabled
      })
    });
    const data = await res.json();

    if (!data.success) {
      updateDownloadModalError(data.message || 'Ошибка запуска скачивания');
      return;
    }

    state.currentJobId = data.jobId;
    trackDownloadProgress(data.jobId, appName, bundleId, iconUrl);
  } catch (err) {
    updateDownloadModalError('Ошибка запуска загрузки');
  }
}

function openDownloadModal(title, subtitle, iconUrl) {
  const modal = document.getElementById('download-modal');
  const titleEl = document.getElementById('dl-modal-title');
  const subEl = document.getElementById('dl-modal-subtitle');
  const iconEl = document.getElementById('dl-modal-icon');
  const fillEl = document.getElementById('dl-progress-bar-fill');
  const percentEl = document.getElementById('dl-progress-percent');
  const bytesEl = document.getElementById('dl-progress-bytes');
  const speedEl = document.getElementById('dl-progress-speed');
  const errorBox = document.getElementById('dl-error-box');
  const openBtn = document.getElementById('dl-modal-open-btn');

  if (titleEl) titleEl.textContent = title || 'Скачивание приложения';
  if (subEl) subEl.textContent = subtitle || 'Подготовка...';
  if (iconEl) {
    if (iconUrl) {
      iconEl.innerHTML = `<img src="${iconUrl}" style="width:64px;height:64px;border-radius:14px;">`;
    } else {
      iconEl.textContent = '📦';
    }
  }

  if (fillEl) fillEl.style.width = '0%';
  if (percentEl) percentEl.textContent = '0%';
  if (bytesEl) bytesEl.textContent = '0 / 0 MB';
  if (speedEl) speedEl.textContent = '—';
  if (errorBox) errorBox.style.display = 'none';
  if (openBtn) openBtn.style.display = 'none';

  // Reset steps
  document.querySelectorAll('.dl-step').forEach(step => step.className = 'dl-step');
  const stepLicense = document.getElementById('step-license');
  if (stepLicense) stepLicense.classList.add('active');

  if (modal) modal.style.display = 'flex';
}

function updateDownloadModalError(errMsg) {
  const errorBox = document.getElementById('dl-error-box');
  const errorText = document.getElementById('dl-error-text');
  if (errorBox && errorText) {
    errorText.textContent = errMsg;
    errorBox.style.display = 'block';
  }
}

function closeDownloadModal() {
  const modal = document.getElementById('download-modal');
  if (modal) modal.style.display = 'none';
}

function trackDownloadProgress(jobId, appName, bundleId, iconUrl) {
  const fillEl = document.getElementById('dl-progress-bar-fill');
  const percentEl = document.getElementById('dl-progress-percent');
  const bytesEl = document.getElementById('dl-progress-bytes');
  const speedEl = document.getElementById('dl-progress-speed');
  const subEl = document.getElementById('dl-modal-subtitle');
  const openBtn = document.getElementById('dl-modal-open-btn');

  const interval = setInterval(async () => {
    try {
      const res = await fetch(`/api/download/status?jobId=${jobId}`);
      const job = await res.json();

      if (!job.id) {
        clearInterval(interval);
        return;
      }

      // Update state
      state.activeDownloads.set(jobId, job);
      updateDownloadsBadge();

      // Update modal progress bar
      const pct = Math.min(100, Math.max(0, job.progress || 0));
      if (fillEl) fillEl.style.width = `${pct}%`;
      if (percentEl) percentEl.textContent = `${pct.toFixed(1)}%`;
      if (bytesEl) {
        bytesEl.textContent = job.totalBytes > 0
          ? `${formatBytes(job.bytesRead)} / ${formatBytes(job.totalBytes)}`
          : `${formatBytes(job.bytesRead)}`;
      }
      if (speedEl) speedEl.textContent = job.speed || '';

      // Update steps
      if (job.status === 'purchasing') {
        document.getElementById('step-license')?.classList.add('active');
        if (subEl) subEl.textContent = 'Получение лицензии App Store...';
      } else if (job.status === 'downloading') {
        document.getElementById('step-license')?.classList.add('completed');
        document.getElementById('step-download')?.classList.add('active');
        if (subEl) subEl.textContent = 'Скачивание зашифрованного пакета...';
      } else if (job.status === 'patching') {
        document.getElementById('step-download')?.classList.add('completed');
        document.getElementById('step-sinf')?.classList.add('active');
        if (subEl) subEl.textContent = 'Применение цифровой подписи (sinf)...';
      } else if (job.status === 'completed') {
        clearInterval(interval);
        document.getElementById('step-sinf')?.classList.add('completed');
        document.getElementById('step-complete')?.classList.add('completed');
        if (subEl) subEl.textContent = 'Пакет .IPA успешно скачан!';
        if (fillEl) fillEl.style.width = '100%';
        if (percentEl) percentEl.textContent = '100%';
        if (openBtn) {
          openBtn.style.display = 'inline-flex';
          openBtn.setAttribute('data-path', job.outputPath || '');
        }

        // Add to history
        addToHistory({
          appName: job.appName || appName || bundleId,
          bundleId: job.bundleId || bundleId,
          version: job.version || '',
          outputPath: job.outputPath,
          bytes: job.totalBytes,
          date: new Date().toLocaleString()
        });

        state.activeDownloads.delete(jobId);
        updateDownloadsBadge();
        showToast(`Файл сохранен: ${job.outputPath}`, 'success');
      } else if (job.status === 'error') {
        clearInterval(interval);
        updateDownloadModalError(job.error || 'Произошла ошибка при скачивании');
        state.activeDownloads.delete(jobId);
        updateDownloadsBadge();
      }
    } catch (err) {
      console.error('Progress poll error:', err);
    }
  }, 500);
}

function openCompletedFolder() {
  const openBtn = document.getElementById('dl-modal-open-btn');
  const path = openBtn ? openBtn.getAttribute('data-path') : '';
  openOutputFolder(path);
}

// ==========================================
// Direct Download Form
// ==========================================

async function handleDirectDownload(e) {
  e.preventDefault();
  const rawId = document.getElementById('direct-app-id').value.trim();
  const platform = document.getElementById('direct-platform').value;
  const versionId = document.getElementById('direct-version-id').value.trim();
  const outputPath = document.getElementById('direct-output-path').value.trim();
  const purchase = document.getElementById('direct-purchase').checked;

  if (!rawId) {
    showToast('Укажите Bundle ID или App ID', 'error');
    return;
  }

  const parsedId = parseAppStoreUrl(rawId);
  const isNumeric = /^\d+$/.test(parsedId);

  const bundleId = isNumeric ? '' : parsedId;
  const appId = isNumeric ? parsedId : 0;

  await startAppDownload(bundleId, appId, platform, parsedId, '', versionId, outputPath);
}

// ==========================================
// Version History Tab
// ==========================================

async function viewAppVersions(bundleId, appId, appName) {
  switchTab('versions');
  // Remember the app name so it can be shown in the version-history header
  // even before /api/versions resolves it.
  state.lastVersionsName = appName || '';
  const input = document.getElementById('versions-input');
  if (input) {
    input.value = bundleId || appId;
    handleFetchVersions(new Event('submit'));
  }
}

// Fetches the Display Version and Release Date for a single build and fills
// the corresponding cells in the version history table.
async function fetchVersionMetadata(bundleId, appId, versionId) {
  try {
    const res = await fetch(`/api/version-metadata?bundleId=${encodeURIComponent(bundleId)}&appId=${encodeURIComponent(appId)}&versionId=${encodeURIComponent(versionId)}`);
    const data = await res.json();
    const dispEl = document.getElementById(`disp-ver-${versionId}`);
    const dateEl = document.getElementById(`date-ver-${versionId}`);
    if (!dispEl && !dateEl) return;

    if (data.success) {
      if (dispEl) dispEl.textContent = data.displayVersion || '—';
      if (dateEl) dateEl.textContent = data.releaseDate || '—';
    } else {
      if (dispEl) dispEl.textContent = '—';
      if (dateEl) dateEl.textContent = '—';
    }
  } catch (err) {
    const dispEl = document.getElementById(`disp-ver-${versionId}`);
    const dateEl = document.getElementById(`date-ver-${versionId}`);
    if (dispEl) dispEl.textContent = '—';
    if (dateEl) dateEl.textContent = '—';
  }
}

async function handleFetchVersions(e) {
  e.preventDefault();
  const rawInput = document.getElementById('versions-input').value.trim();
  if (!rawInput) return;

  const parsed = parseAppStoreUrl(rawInput);
  const isNumeric = /^\d+$/.test(parsed);

  const bundleId = isNumeric ? '' : parsed;
  const appId = isNumeric ? parsed : '';

  const loading = document.getElementById('versions-loading');
  const container = document.getElementById('versions-container');
  const tableBody = document.getElementById('versions-table-body');
  const titleEl = document.getElementById('versions-app-title');
  const badgeEl = document.getElementById('versions-count-badge');
  const bundleEl = document.getElementById('versions-app-bundle');

  loading.style.display = 'block';
  container.style.display = 'none';

  try {
    const res = await fetch(`/api/versions?bundleId=${encodeURIComponent(bundleId)}&appId=${encodeURIComponent(appId)}`);
    const data = await res.json();
    loading.style.display = 'none';

    if (!data.success) {
      showToast(data.message || 'Не удалось получить версии приложения', 'error');
      return;
    }

    const versions = data.externalVersionIdentifiers || [];
    const appName = data.name || state.lastVersionsName || '';
    const appNameEsc = (appName || '').replace(/'/g, "\\'");
    titleEl.textContent = appName || data.bundleID || parsed;
    bundleEl.textContent = `Bundle ID: ${data.bundleID || parsed}`;
    badgeEl.textContent = `${versions.length} версий`;
    tableBody.innerHTML = '';

    // Show versions in reverse order (newest first)
    const resolvedBundleId = data.bundleID || bundleId;
    const resolvedAppId = Number(appId) || 0;
    const reversed = [...versions].reverse();
    reversed.forEach((vId, idx) => {
      const row = document.createElement('tr');
      row.innerHTML = `
        <td><code>${vId}</code> ${idx === 0 ? '<span class="badge badge-success">Последняя</span>' : ''}</td>
        <td id="disp-ver-${vId}">…</td>
        <td id="date-ver-${vId}">…</td>
        <td>
          <button class="btn btn-primary btn-sm" onclick="startAppDownload('${resolvedBundleId}', ${resolvedAppId}, 'iphone', '${appNameEsc || resolvedBundleId}', '', '${vId}')">
            ⬇️ Скачать
          </button>
        </td>
      `;
      tableBody.appendChild(row);
    });

    container.style.display = 'block';

    // Fill in the Display Version and Release Date columns asynchronously.
    // Fetch a few at a time (bounded concurrency) so Apple is not hammered by
    // dozens of parallel range requests, which makes each one slower overall.
    const METADATA_CONCURRENCY = 5;
    let nextIndex = 0;
    async function worker() {
      while (nextIndex < reversed.length) {
        const vId = reversed[nextIndex++];
        await fetchVersionMetadata(resolvedBundleId, resolvedAppId, vId);
      }
    }
    const workers = [];
    for (let i = 0; i < Math.min(METADATA_CONCURRENCY, reversed.length); i++) {
      workers.push(worker());
    }
    await Promise.all(workers);
  } catch (err) {
    loading.style.display = 'none';
    showToast('Ошибка связи с сервером', 'error');
  }
}

// ==========================================
// Downloads & History Manager
// ==========================================

function updateDownloadsBadge() {
  const badge = document.getElementById('downloads-badge');
  const count = state.activeDownloads.size;
  if (badge) {
    if (count > 0) {
      badge.textContent = count;
      badge.style.display = 'inline-block';
    } else {
      badge.style.display = 'none';
    }
  }
}

function addToHistory(item) {
  state.downloadHistory.unshift(item);
  if (state.downloadHistory.length > 50) {
    state.downloadHistory = state.downloadHistory.slice(0, 50);
  }
  localStorage.setItem('ipatool_download_history', JSON.stringify(state.downloadHistory));
  renderDownloadsTab();
}

function renderDownloadsTab() {
  const historyList = document.getElementById('downloads-history-list');
  const noHistory = document.getElementById('no-history-downloads');
  if (!historyList) return;

  if (state.downloadHistory.length === 0) {
    historyList.innerHTML = '';
    if (noHistory) historyList.appendChild(noHistory);
    return;
  }

  historyList.innerHTML = '';
  state.downloadHistory.forEach((item, idx) => {
    const el = document.createElement('div');
    el.className = 'history-item';
    const fileName = item.outputPath ? item.outputPath.split(/[\\/]/).pop() : `${item.bundleId}.ipa`;

    el.innerHTML = `
      <div class="history-info">
        <div class="history-file-icon">📦</div>
        <div>
          <div class="history-filename">${item.appName || fileName}</div>
          <div class="history-meta">${fileName} • ${formatBytes(item.bytes)} • ${item.date || ''}</div>
        </div>
      </div>
      <div class="app-actions">
        <button class="btn btn-outline btn-sm" onclick="openOutputFolder('${item.outputPath ? item.outputPath.replace(/\\/g, '\\\\') : ''}')">
          📂 В проводнике
        </button>
        <button class="btn btn-outline btn-sm" onclick="copyToClipboard('${item.outputPath ? item.outputPath.replace(/\\/g, '\\\\') : ''}')">
          📋 Путь
        </button>
        <button class="icon-btn-inline" onclick="removeFromHistory(${idx})" title="Удалить из списка">
          🗑️
        </button>
      </div>
    `;
    historyList.appendChild(el);
  });
}

function removeFromHistory(idx) {
  state.downloadHistory.splice(idx, 1);
  localStorage.setItem('ipatool_download_history', JSON.stringify(state.downloadHistory));
  renderDownloadsTab();
}

function refreshDownloadsHistory() {
  renderDownloadsTab();
  showToast('Список обновлен', 'info');
}

async function openOutputFolder(path = '') {
  try {
    const res = await fetch('/api/open-folder', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path })
    });
    const data = await res.json();
    if (!data.success) {
      showToast(data.message || 'Не удалось открыть папку', 'error');
    }
  } catch (err) {
    showToast('Ошибка при открытии папки', 'error');
  }
}

// ==========================================
// Initialization
// ==========================================

document.addEventListener('DOMContentLoaded', () => {
  applyTheme();
  applyLanguage();
  fetchStatus();
  checkICloudStatus();
  renderDownloadsTab();

  // Search input typing listeners
  const searchInput = document.getElementById('search-input');
  const clearBtn = document.getElementById('search-clear-btn');
  if (searchInput && clearBtn) {
    searchInput.addEventListener('input', () => {
      clearBtn.style.display = searchInput.value.length > 0 ? 'block' : 'none';
    });
  }
});
