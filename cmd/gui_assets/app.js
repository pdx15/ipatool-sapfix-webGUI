/**
 * ipatool GUI — Interactive Web Application Logic
 * Supports Russian (default) and English localization
 */

// Application State
const state = {
  lang: localStorage.getItem('ipatool_lang') || 'ru',
  theme: localStorage.getItem('ipatool_theme') || 'dark',
  activeTab: 'search',
  os: null, // 'windows' | 'darwin' | 'linux' | ..., from /api/status
  account: null,
  isAuthenticated: false,
  activeDownloads: new Map(), // jobId -> job object
  downloadHistory: JSON.parse(localStorage.getItem('ipatool_download_history') || '[]'),
  lastPendingLogin: null, // { email, password } for 2FA retry
  currentJobId: null,
  lastVersionsName: '', // app name remembered when opening version history from a search card
  isDownloading: false, // prevent double-click on download buttons
  completedJobIds: new Set() // prevent duplicate toasts/handlers for same job
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
    tab_install: 'Установка на устройство',
    tab_donate: 'Донат',
    donate_title: 'Поддержать проект',
    donate_desc: 'Если вы хотите безвозмездно поддержать проект, то можно это сделать по ссылке ниже или через QR-код:',
    donate_button: 'Поддержать проект через Cloudtips',
    donate_qr_caption: 'QR-код для поддержки проекта',
    install_devices_title: 'Подключенные устройства iOS',
    install_devices_desc: 'Обнаруживаются автоматически через libimobiledevice (idevice_id / idevicedeviceinfo)',
    install_devices_scanning: 'Поиск подключенных устройств...',
    install_no_devices: 'Подключите iPhone/iPad по кабелю и нажмите «Обновить»',
    install_tools_missing_title: 'Не найден ideviceinstaller',
    install_tools_missing_desc: 'Для установки нужен ideviceinstaller из libimobiledevice. На macOS: brew install libimobiledevice. На Windows установите iDevice Suite / сборку libimobiledevice и положите ideviceinstaller.exe в PATH.',
    install_driver_check_title: 'Драйвер Apple Mobile Device Support',
    install_driver_ok: 'Драйвер Apple Mobile Device Support найден. USB-подключение готово к работе.',
    install_driver_missing: 'Драйвер (служба) Apple Mobile Device Support не найден. Установите iTunes, чтобы получить его, или обновите драйвер USB для устройства.',
    install_driver_download: 'Скачать Apple Mobile Device Support',
    install_driver_itunes: 'Скачать iTunes',
    install_driver_unknown: 'Проверка драйвера недоступна на этой системе.',
    install_upload_title: 'Выберите файл .IPA',
    install_upload_desc: 'Выберите скачанный .IPA, укажите устройство и установите его на iPhone/iPad',
    install_device_label: 'Устройство:',
    install_device_select_placeholder: '— выберите устройство —',
    install_file_label: 'Файл .IPA:',
    install_dropzone_text: 'Нажмите, чтобы выбрать .ipa файл',
    install_button: 'Установить на устройство',
    install_progress_title: 'Установка...',
    install_log_title: 'Журнал ideviceinstaller',
    install_started_toast: 'Установка началась — прогресс ниже',
    install_file_selected: 'Выбран файл:',
    install_needs_file: 'Выберите файл .IPA',
    install_needs_device: 'Выберите подключенное устройство',
    install_devices_refresh_error: 'Не удалось получить список устройств',
    install_completed: 'Приложение успешно установлено!',
    install_error: 'Ошибка установки',
    install_status_queued: 'В очереди',
    install_status_installing: 'Установка',
    install_status_completed: 'Готово',
    install_status_error: 'Ошибка',
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
    search_removed_title: 'Удалённые из App Store (доступны по ID)',
    search_official_title: 'Результаты App Store',
    removed_badge: 'Удалено из App Store',
    results_found: 'Найдено приложений: {count}',
    removed_found: 'Найдено удалённых: {count}',
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
    version_col_select: 'Выбрать',
    min_ios_badge_title: 'Минимальная поддерживаемая версия iOS',
    batch_version_pick_hint: 'Отметьте версию, чтобы скачать именно её вместо последней (по кнопке «Скачать выбранные» вверху). Отметка снимает галочку с карточки.',
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
    login_test_btn: 'Войти в Apple ID SKIP',
    login_test_hint: 'Если при авторизации через «Войти в Apple ID» возникают ошибки (HTTP 403, «An unknown error has occurred», ошибки GSA/SRP или проблемы с 2FA), используйте альтернативный метод авторизации через «Войти в Apple ID SKIP».',
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
    copied_toast: 'Скопировано в буфер обмена',
    tab_batch: 'Массовая загрузка',
    batch_title: 'Массовая проверка и загрузка',
    batch_desc: 'Загрузите текстовый файл со списком приложений (название и App ID). Каждый ID будет проверен через Прямую загрузку: приложения, ранее установленные на вашем Apple ID, скачиваются, а те, которых у вас никогда не было (ошибка «license is required»), будут отфильтрованы.',
    batch_file_label: 'Файл со списком приложений (.txt):',
    batch_file_hint: 'Каждая строка: название и числовой App ID, например 1Password 7: 568903335. Поддерживаются также просто ID, ссылки на App Store и разделители «:», «-», «;», Tab или пробел.',
    batch_paste_label: 'Или вставьте список текстом:',
    batch_parsed_count: 'Распознано приложений: {count}. Нажмите «Проверить по Apple ID», чтобы прогнать каждое через Прямую загрузку.',
    batch_check_btn: 'Проверить по Apple ID',
    batch_check_progress_title: 'Проверка приложений через Прямую загрузку...',
    batch_check_progress_text: 'Проверено {done} из {total} (каждый прогон = {pct}% работы).',
    batch_results_title: 'Результаты проверки',
    batch_results_summary: 'Доступно для загрузки: {available} из {total}. Отфильтровано: {filtered}.',
    batch_select_all: 'Выбрать все',
    batch_select_none: 'Снять все',
    batch_download_selected: 'Скачать выбранные',
    batch_save_list: 'Сохранить список (.txt)',
    batch_save_toast: 'Список сохранён в текстовый файл',
    batch_filtered_title: 'Отфильтрованы (нет лицензии на Apple ID):',
    batch_filtered_error_title: 'Другие ошибки:',
    batch_version_history: 'История версий',
    batch_version_history_close: 'Скрыть историю',
    batch_versions_loading: 'Получение данных о версиях...',
    batch_no_versions: 'Список версий недоступен',
    batch_latest_badge: 'Последняя',
    batch_select_hint: 'Отметьте приложения и нажмите «Скачать выбранные»:',
    batch_download_progress_title: 'Массовое скачивание выбранных приложений...',
    batch_download_progress_done: 'Скачано {done} из {total}. Ошибок: {errors}.',
    batch_download_status_queued: 'В очереди',
    batch_download_status_purchasing: 'Лицензия',
    batch_download_status_downloading: 'Скачивание',
    batch_download_status_patching: 'Подпись (sinf)',
    batch_download_status_completed: 'Готово',
    batch_download_status_error: 'Ошибка',
    batch_download_done_title: 'Массовая загрузка завершена',
    batch_download_started_toast: 'Загрузка началась — прогресс ниже',
    batch_check_running: 'Проверка уже выполняется',
    batch_no_items: 'Не удалось распознать App ID в списке',
    batch_need_auth: 'Сначала необходимо войти в Apple ID во вкладке «Аккаунт»',
    batch_no_selected: 'Выберите хотя бы одно приложение',
    batch_download_single: 'Скачать'
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
    tab_install: 'Install to device',
    tab_donate: 'Donate',
    donate_title: 'Support the project',
    donate_desc: 'If you would like to support the project free of charge, you can do so via the link below or by scanning the QR code:',
    donate_button: 'Support the project via Cloudtips',
    donate_qr_caption: 'QR code to support the project',
    install_devices_title: 'Connected iOS devices',
    install_devices_desc: 'Detected automatically with libimobiledevice (idevice_id / idevicedeviceinfo)',
    install_devices_scanning: 'Scanning for connected devices...',
    install_no_devices: 'Connect an iPhone/iPad by cable and press "Refresh"',
    install_tools_missing_title: 'ideviceinstaller not found',
    install_tools_missing_desc: 'Installing requires ideviceinstaller from libimobiledevice. On macOS: brew install libimobiledevice. On Windows install iDevice Suite / a libimobiledevice build and put ideviceinstaller.exe on PATH.',
    install_driver_check_title: 'Apple Mobile Device Support driver',
    install_driver_ok: 'Apple Mobile Device Support driver found. USB connection is ready.',
    install_driver_missing: 'Apple Mobile Device Support driver/service not found. Install iTunes (which bundles it) or update the Apple USB driver for the device.',
    install_driver_download: 'Download Apple Mobile Device Support',
    install_driver_itunes: 'Download iTunes',
    install_driver_unknown: 'Driver check is not available on this system.',
    install_upload_title: 'Choose an .IPA file',
    install_upload_desc: 'Select a downloaded .IPA, choose the device and install it onto iPhone/iPad',
    install_device_label: 'Device:',
    install_device_select_placeholder: '— select a device —',
    install_file_label: 'IPA file:',
    install_dropzone_text: 'Click to select an .ipa file',
    install_button: 'Install on device',
    install_progress_title: 'Installing...',
    install_log_title: 'ideviceinstaller log',
    install_started_toast: 'Installation started — progress is below',
    install_file_selected: 'Selected file:',
    install_needs_file: 'Select an .ipa file',
    install_needs_device: 'Select a connected device',
    install_devices_refresh_error: 'Failed to get device list',
    install_completed: 'App installed successfully!',
    install_error: 'Installation failed',
    install_status_queued: 'Queued',
    install_status_installing: 'Installing',
    install_status_completed: 'Done',
    install_status_error: 'Error',
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
    search_removed_title: 'Removed from the App Store (available by ID)',
    search_official_title: 'App Store Results',
    removed_badge: 'Removed from App Store',
    results_found: 'Found apps: {count}',
    removed_found: 'Found removed apps: {count}',
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
    version_col_select: 'Select',
    min_ios_badge_title: 'Minimum supported iOS version',
    batch_version_pick_hint: 'Tick a version to download it instead of the latest (via the "Download selected" button above). Ticking it clears the card checkbox.',
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
    login_test_btn: 'Sign In Apple ID SKIP',
    login_test_hint: 'If you get errors while signing in via "Sign In" (e.g. HTTP 403, "An unknown error has occurred", GSA/SRP login failures, or 2FA code issues), use the alternative method "Sign In Apple ID SKIP".',
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
    copied_toast: 'Copied to clipboard',
    tab_batch: 'Mass Download',
    batch_title: 'Mass Check & Download',
    batch_desc: 'Upload a text file with a list of apps (name and App ID). Each ID is run through the Direct Download path: apps previously installed on your Apple ID work, while apps you never installed (the "license is required" error) are filtered out.',
    batch_file_label: 'App list file (.txt):',
    batch_file_hint: 'One entry per line: a name and a numeric App ID, e.g. 1Password 7: 568903335. Plain IDs, App Store URLs and separators ":", "-", ";", Tab or space are also supported.',
    batch_paste_label: 'Or paste the list as text:',
    batch_parsed_count: 'Recognized apps: {count}. Press "Check with Apple ID" to run each one through Direct Download.',
    batch_check_btn: 'Check with Apple ID',
    batch_check_progress_title: 'Checking apps through Direct Download...',
    batch_check_progress_text: 'Checked {done} of {total} (each pass = {pct}% of the work).',
    batch_results_title: 'Check Results',
    batch_results_summary: 'Available for download: {available} of {total}. Filtered out: {filtered}.',
    batch_select_all: 'Select all',
    batch_select_none: 'Select none',
    batch_download_selected: 'Download selected',
    batch_save_list: 'Save list (.txt)',
    batch_save_toast: 'List saved to a text file',
    batch_filtered_title: 'Filtered out (no license on Apple ID):',
    batch_filtered_error_title: 'Other errors:',
    batch_version_history: 'Version history',
    batch_version_history_close: 'Hide history',
    batch_versions_loading: 'Fetching version data...',
    batch_no_versions: 'Version list unavailable',
    batch_latest_badge: 'Latest',
    batch_select_hint: 'Check the apps and press "Download selected":',
    batch_download_progress_title: 'Mass downloading selected apps...',
    batch_download_progress_done: 'Downloaded {done} of {total}. Errors: {errors}.',
    batch_download_status_queued: 'Queued',
    batch_download_status_purchasing: 'License',
    batch_download_status_downloading: 'Downloading',
    batch_download_status_patching: 'Signing (sinf)',
    batch_download_status_completed: 'Done',
    batch_download_status_error: 'Error',
    batch_download_done_title: 'Mass download finished',
    batch_download_started_toast: 'Download started — progress is below',
    batch_check_running: 'A check is already running',
    batch_no_items: 'Could not recognize an App ID in the list',
    batch_need_auth: 'Sign in to your Apple ID in the "Account" tab first',
    batch_no_selected: 'Select at least one app',
    batch_download_single: 'Download'
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
    startActiveDownloadsPolling();
  } else {
    stopActiveDownloadsPolling();
  }

  if (tabName === 'install') {
    refreshInstallDevices();
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

// Shows/hides platform-specific pieces of the UI once the server-reported OS
// is known. The iCloud-for-Windows card is hidden on macOS, and the alternative
// "Войти в Apple ID SKIP" (GSA -> MZFinance) button is shown only on macOS.
function applyPlatformVisibility() {
  const isMac = state.os === 'darwin';

  const icloudCard = document.getElementById('icloud-presence-card');
  if (icloudCard) icloudCard.style.display = isMac ? 'none' : '';

  const testBtn = document.getElementById('login-test-btn');
  const testHint = document.getElementById('login-test-hint');
  if (testBtn) testBtn.style.display = isMac ? 'inline-flex' : 'none';
  if (testHint) testHint.style.display = isMac ? '' : 'none';
}

async function checkICloudStatus() {
  // The iCloud presence check is a Windows-only feature; skip it on macOS.
  if (state.os === 'darwin') return;

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
    state.os = data.os || null;

    applyPlatformVisibility();
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

// Handle Login (standard path: GSA -> native/fast -> MZFinance)
async function handleLogin(e) {
  e.preventDefault();
  const email = document.getElementById('login-email').value.trim();
  const password = document.getElementById('login-password').value;
  const submitBtn = document.getElementById('login-submit-btn');

  if (!email || !password) {
    showToast('Заполните Email и Пароль', 'error');
    return;
  }

  const originalHtml = submitBtn.innerHTML;
  submitBtn.disabled = true;
  submitBtn.textContent = 'Авторизация...';

  await submitLogin('/api/auth/login', email, password, '', submitBtn, originalHtml);
}

// Handle Test Login (macOS-only diagnostic: GSA -> MZFinance directly)
async function handleTestLogin(e) {
  e.preventDefault();
  const email = document.getElementById('login-email').value.trim();
  const password = document.getElementById('login-password').value;
  const submitBtn = document.getElementById('login-test-btn');

  if (!email || !password) {
    showToast('Заполните Email и Пароль', 'error');
    return;
  }

  const originalHtml = submitBtn.innerHTML;
  submitBtn.disabled = true;
  submitBtn.textContent = 'Авторизация (MZFinance)...';

  await submitLogin('/api/auth/login/mzfinance', email, password, '', submitBtn, originalHtml);
}

// Shared login submission for both the standard and the test (MZFinance) paths.
// Remembering the endpoint in lastPendingLogin lets the 2FA retry reuse the
// exact same flow that was in progress.
async function submitLogin(endpoint, email, password, authCode, submitBtn, originalHtml) {
  try {
    const res = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password, authCode })
    });
    const data = await res.json();

    if (data.authCodeRequired) {
      // 2FA required! Open 2FA modal, remembering which flow triggered it.
      state.lastPendingLogin = { email, password, endpoint };
      open2FAModal();
      showToast('Требуется код двухфакторной аутентификации', 'info');
    } else if (data.anisetteRequired) {
      // Windows GSA login needs a locally installed & signed-in iCloud to
      // produce anisette headers. Show the precise reason returned by the
      // backend so the user knows exactly which check failed.
      showToast('Ошибка iCloud (anisette): ' + (data.message || 'проверьте установку iCloud'), 'error');
      switchTab('account');
    } else if (data.success) {
      close2FAModal();
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
    if (submitBtn) {
      submitBtn.disabled = false;
      if (originalHtml) submitBtn.innerHTML = originalHtml;
    }
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

  const originalHtml = submitBtn.innerHTML;
  submitBtn.disabled = true;
  submitBtn.textContent = 'Проверка...';

  // Reuse the same login flow that triggered the 2FA prompt (standard or the
  // macOS MZFinance test path), so the retry does not switch flows.
  const endpoint = state.lastPendingLogin.endpoint || '/api/auth/login';
  await submitLogin(
    endpoint,
    state.lastPendingLogin.email,
    state.lastPendingLogin.password,
    code,
    submitBtn,
    originalHtml
  );
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

// Fetches both result groups for a search: the apps that were removed from the
// App Store but stay downloadable by ID (the Apps_ID_List.txt catalog) and the
// official App Store results. /api/search/all returns them in the order the UI
// renders them — removed apps first — so the grouping does not depend on which
// of two parallel responses happens to arrive earlier. If the merged endpoint
// is unavailable (older backend), fall back to the two separate endpoints.
async function fetchSearchResults(term, platform, limit) {
  const termParam = `term=${encodeURIComponent(term)}`;
  try {
    const res = await fetch(`/api/search/all?${termParam}&platform=${platform}&limit=${limit}`);
    if (res.ok) {
      const data = await res.json();
      if (data && data.success) {
        return {
          apps: data.official?.results || [],
          removedApps: data.removed?.results || [],
          message: data.official?.error || ''
        };
      }
    }
  } catch (e) { /* merged endpoint unavailable — use the separate ones */ }

  const [searchRes, removedRes] = await Promise.all([
    fetch(`/api/search?${termParam}&platform=${platform}&limit=${limit}`),
    fetch(`/api/removed-apps?${termParam}&limit=${limit}`)
  ]);
  const data = await searchRes.json();
  if (!data.success) {
    return { apps: [], removedApps: [], message: data.message || 'Ошибка поиска в App Store' };
  }

  let removedData = { success: false, results: [] };
  try {
    removedData = await removedRes.json();
  } catch (e) { /* removed list endpoint unavailable — treat as empty */ }

  return {
    apps: data.results || [],
    removedApps: (removedData && removedData.success) ? (removedData.results || []) : [],
    message: ''
  };
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
    const { apps, removedApps, message } = await fetchSearchResults(term, platform, limit);

    loadingEl.style.display = 'none';

    if (message) {
      showToast(message, 'error');
      if (removedApps.length === 0) {
        emptyEl.style.display = 'block';
        return;
      }
    }

    const officialSection = document.getElementById('official-results-section');
    const officialHeader = document.getElementById('official-results-header');
    const removedSection = document.getElementById('removed-results-section');
    const removedGrid = document.getElementById('removed-results-grid');
    const removedCount = document.getElementById('removed-results-count');

    resultsGrid.innerHTML = '';
    removedGrid.innerHTML = '';

    // Removed apps are rendered first (their section comes before the official
    // one in index.html), so they are the first thing you see after a search.
    if (removedApps.length > 0) {
      removedSection.style.display = 'block';
      removedCount.textContent = batchText('removed_found', { count: removedApps.length });
      removedApps.forEach(app => removedGrid.appendChild(createRemovedAppCard(app, platform)));
    } else {
      removedSection.style.display = 'none';
    }

    if (apps.length > 0) {
      officialSection.style.display = 'block';
      resultsCount.textContent = batchText('results_found', { count: apps.length });
      apps.forEach(app => resultsGrid.appendChild(createAppCard(app, platform)));
    } else {
      officialSection.style.display = 'none';
    }

    // The divider only makes sense when both groups are on screen; otherwise
    // the official results would start with a stray horizontal rule.
    if (officialHeader) {
      officialHeader.classList.toggle('results-header-divider', removedApps.length > 0 && apps.length > 0);
    }

    if (apps.length === 0 && removedApps.length === 0) {
      noResultsEl.style.display = 'block';
      resultsWrapper.style.display = 'none';
      return;
    }

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

// Renders a card for an app that was removed from the App Store but is still
// downloadable by its numeric App ID (from the Apps_ID_List.txt catalog).
// Unlike official results there is no artwork or bundle ID, so only App ID
// based actions (download / version history) are offered.
function createRemovedAppCard(app, platform) {
  const card = document.createElement('div');
  card.className = 'app-card removed-app-card';

  const dict = i18n[state.lang] || i18n.ru;
  const name = app.name || String(app.appId);
  const nameEsc = String(name).replace(/'/g, "\\'");
  const appId = Number(app.appId) || 0;

  card.innerHTML = `
    <div class="app-main-info">
      <div class="app-icon-fallback removed-icon">🗄️</div>
      <div class="app-meta">
        <h3 class="app-name" title="${batchEscapeHtml(name)}">${batchEscapeHtml(name)}</h3>
        <p class="app-artist">App ID: <code>${appId}</code></p>
        <div class="app-badges">
          <span class="badge badge-removed">${dict.removed_badge}</span>
          <span class="badge">${platform.toUpperCase()}</span>
        </div>
      </div>
    </div>

    <div class="app-identifiers">
      <div class="id-row">
        <span class="id-label">App ID:</span>
        <span class="id-value">
          <code>${appId}</code>
          <button type="button" class="copy-icon-btn" onclick="copyToClipboard('${appId}')" title="Скопировать">📋</button>
        </span>
      </div>
    </div>

    <div class="app-actions">
      <button class="btn btn-primary" onclick="startAppDownload('', ${appId}, '${platform}', '${nameEsc}', '')">
        ⬇️ ${dict.download_btn}
      </button>
      <button class="btn btn-outline" onclick="viewAppVersions('', ${appId}, '${nameEsc}')" title="История версий">
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

  // Prevent double-click
  if (state.isDownloading) {
    return;
  }
  state.isDownloading = true;

  // Open download progress modal
  openDownloadModal(appName, bundleId, iconUrl);

  try {
    const res = await fetch('/api/download', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        bundleId: bundleId || '',
        appId: appId ? parseInt(appId, 10) : 0,
        appName: appName || '',
        platform: platform || 'iphone',
        externalVersionId: versionId || '',
        outputPath: outputPath || '',
        purchase: true // auto purchase enabled
      })
    });
    const data = await res.json();

    if (!data.success) {
      state.isDownloading = false;
      updateDownloadModalError(data.message || 'Ошибка запуска скачивания');
      return;
    }

    state.currentJobId = data.jobId;
    trackDownloadProgress(data.jobId, appName, bundleId, iconUrl);
  } catch (err) {
    state.isDownloading = false;
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

  let handling = false; // Prevent parallel callback execution

  const interval = setInterval(async () => {
    // Skip if already handling or job already fully processed
    if (handling || state.completedJobIds.has(jobId)) return;
    handling = true;

    try {
      const res = await fetch(`/api/download/status?jobId=${jobId}`);
      const job = await res.json();

      if (!job.id) {
        clearInterval(interval);
        handling = false;
        return;
      }

      // Update state
      state.activeDownloads.set(jobId, job);
      updateDownloadsBadge();
      renderActiveDownloadsTab();

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

        // Skip if already processed (prevents duplicate toasts from parallel callbacks)
        if (state.completedJobIds.has(jobId)) {
          handling = false;
          return;
        }
        state.completedJobIds.add(jobId);

        document.getElementById('step-sinf')?.classList.add('completed');
        document.getElementById('step-complete')?.classList.add('completed');
        if (subEl) subEl.textContent = 'Пакет .IPA успешно скачан!';
        if (fillEl) fillEl.style.width = '100%';
        if (percentEl) percentEl.textContent = '100%';
        if (openBtn) {
          openBtn.style.display = 'inline-flex';
          openBtn.setAttribute('data-path', job.outputPath || '');
        }

        addToHistory({
          appName: job.appName || appName || bundleId,
          bundleId: job.bundleId || bundleId,
          version: job.version || '',
          outputPath: job.outputPath,
          bytes: job.totalBytes,
          date: new Date().toLocaleString()
        });

        state.activeDownloads.delete(jobId);
        state.isDownloading = false;
        updateDownloadsBadge();
        renderActiveDownloadsTab();
        showToast(`Файл сохранен: ${job.outputPath}`, 'success');
        handling = false;
        return;
      } else if (job.status === 'error') {
        clearInterval(interval);
        updateDownloadModalError(job.error || 'Произошла ошибка при скачивании');
        state.activeDownloads.delete(jobId);
        state.isDownloading = false;
        updateDownloadsBadge();
        renderActiveDownloadsTab();
        handling = false;
        return;
      }

      handling = false;
    } catch (err) {
      console.error('Progress poll error:', err);
      handling = false;
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
      if (dispEl) dispEl.innerHTML = renderDisplayVersionCell(data.displayVersion, data.minimumOSVersion);
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

// Builds the inner HTML of a "Display Version" table cell: the app version and,
// when known, a small "iOS x.y" badge showing the minimum supported iOS
// version (read from the IPA Info.plist MinimumOSVersion key).
function renderDisplayVersionCell(displayVersion, minimumOSVersion) {
  const version = batchEscapeHtml(displayVersion || '—');
  const minOS = (minimumOSVersion || '').trim();
  if (!minOS) return version;
  const title = batchText('min_ios_badge_title');
  return `${version} <span class="badge badge-ios" title="${batchEscapeHtml(title)}">iOS ${batchEscapeHtml(minOS)}</span>`;
}

async function handleFetchVersions(e) {
  e.preventDefault();
  const rawInput = document.getElementById('versions-input').value.trim();
  if (!rawInput) return;

  // Reset lastVersionsName to avoid showing name from previous search
  state.lastVersionsName = '';

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

// Renders the "Активные загрузки" section on the Downloads tab from the
// client-side job map. Terminal jobs are removed from the map by their own
// pollers, so this list only shows jobs that are still running.
function renderActiveDownloadsTab() {
  const listEl = document.getElementById('active-downloads-list');
  const noEl = document.getElementById('no-active-downloads');
  if (!listEl) return;

  const jobs = Array.from(state.activeDownloads.values())
    .sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));
  const active = jobs.filter(j => j.status !== 'completed' && j.status !== 'error');

  // If nothing is active, restore the built-in empty state placeholder.
  if (active.length === 0) {
    listEl.innerHTML = '';
    if (noEl) listEl.appendChild(noEl);
    return;
  }

  listEl.innerHTML = '';
  active.forEach(job => {
    const pct = Math.min(100, Math.max(0, job.progress || 0));
    const statusLabel = (i18n[state.lang] || i18n.ru)[`batch_download_status_${job.status}`] || job.status;
    const el = document.createElement('div');
    el.className = 'active-download-card';
    el.innerHTML = `
      <div class="active-dl-header">
        <div class="active-dl-title" title="${(job.appName || job.bundleId || job.id || '').replace(/"/g, '&quot;')}">
          ${(job.appName || job.bundleId || job.id || '').replace(/</g, '&lt;')}
        </div>
        <span class="badge ${job.status === 'error' ? 'badge-error' : 'badge-version'}">${statusLabel}</span>
      </div>
      <div class="dl-progress-bar-bg">
        <div class="dl-progress-bar-fill" style="width:${pct}%"></div>
      </div>
      <div class="dl-progress-stats">
        <span>${pct.toFixed(1)}%</span>
        <span>${job.totalBytes > 0 ? `${formatBytes(job.bytesRead)} / ${formatBytes(job.totalBytes)}` : (job.bytesRead ? formatBytes(job.bytesRead) : '')} ${job.speed || ''}</span>
      </div>
      ${job.status === 'error' ? `<div class="text-secondary" style="margin-top:6px">${(job.error || '').replace(/</g, '&lt;')}</div>` : ''}
    `;
    listEl.appendChild(el);
  });
}

// Fetches active jobs from the server so the Downloads tab is also correct
// after a page reload or for jobs started from other tabs (e.g. batch).
async function refreshActiveDownloads() {
  try {
    const res = await fetch('/api/downloads/active');
    const data = await res.json();
    if (!data.success) return;

    const live = new Map();
    (data.jobs || []).forEach(job => {
      if (job && job.id) live.set(job.id, job);
    });

    // Merge server state; keep locally-known jobs that are not yet reported
    // (e.g. just queued) and drop nothing while they are still active.
    live.forEach((job, id) => state.activeDownloads.set(id, job));
    const activeIds = new Set(live.keys());
    state.activeDownloads.forEach((job, id) => {
      // Once a job is terminal, its poller removes it; if the server no longer
      // lists it and it is not terminal here, drop the stale entry.
      if (!activeIds.has(id) && job.status !== 'completed' && job.status !== 'error') {
        state.activeDownloads.delete(id);
      }
    });

    updateDownloadsBadge();
    renderActiveDownloadsTab();
  } catch (err) {
    console.error('Active downloads refresh error:', err);
  }
}

let activeDownloadsTimer = null;

function startActiveDownloadsPolling() {
  if (activeDownloadsTimer) return;
  refreshActiveDownloads();
  activeDownloadsTimer = setInterval(refreshActiveDownloads, 2000);
}

function stopActiveDownloadsPolling() {
  if (activeDownloadsTimer) {
    clearInterval(activeDownloadsTimer);
    activeDownloadsTimer = null;
  }
}

function addToHistory(item) {
  // Check for duplicates by outputPath (if provided) within last 10 seconds
  const now = Date.now();
  const isDuplicate = state.downloadHistory.some(existing => {
    // Match by outputPath if both have it
    if (item.outputPath && existing.outputPath && item.outputPath === existing.outputPath) {
      // Check if existing entry is recent (within 10 seconds)
      if (existing._timestamp) {
        const age = now - existing._timestamp;
        if (age < 10000) return true;
      }
    }
    return false;
  });
  
  if (isDuplicate) {
    return; // Skip duplicate
  }
  
  // Add timestamp for duplicate detection
  item._timestamp = now;
  
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
// Batch Mass Download Tab
// ==========================================

const batchState = {
  items: [],              // parsed [{ appId, name }]
  results: null,          // last check job payload
  selected: new Set(),    // appIds whose card checkbox is ticked (download latest)
  selectedVersions: {},   // appId -> a specific versionId picked in version history
  addedToHistory: new Set(),
  checkPoll: null,
  downloadPoll: null
};

// Parse a txt list into [{appId, name}]. Supports "Name: 568903335",
// plain IDs, App Store URLs and various separators.
function parseBatchList(text) {
  const items = [];
  const seen = new Set();

  (text || '').split(/\r?\n/).forEach(raw => {
    const line = raw.trim();
    if (!line) return;

    const urlMatch = line.match(/id(\d{4,})/i);
    let idStr = urlMatch ? urlMatch[1] : null;
    let namePart = line;

    if (idStr) {
      namePart = line.replace(new RegExp('id' + idStr + '\\b', 'i'), '').trim();
      if (/^https?:\/\/|apps\.apple\.com|\//i.test(namePart)) {
        namePart = '';
      }
    } else {
      const nums = line.match(/\d{4,}/g);
      if (!nums || nums.length === 0) return;
      idStr = nums[nums.length - 1];
      namePart = line.slice(0, line.lastIndexOf(idStr)).trim();
    }

    const appId = parseInt(idStr, 10);
    if (!appId || seen.has(appId)) return;
    seen.add(appId);

    namePart = namePart.replace(/^[\s\-–—:;,.()]+|[\s\-–—:;,.()]+$/g, '').trim();
    items.push({ appId, name: namePart || String(appId) });
  });

  return items;
}

function batchText(key, vars) {
  let text = (i18n[state.lang] || i18n.ru)[key] || key;
  if (vars) {
    Object.keys(vars).forEach(k => {
      text = text.replaceAll(`{${k}}`, vars[k]);
    });
  }
  return text;
}

function batchEscapeHtml(str) {
  return String(str || '')
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function updateBatchParsedSummary() {
  const summary = document.getElementById('batch-parsed-summary');
  const countEl = document.getElementById('batch-parsed-count');
  const input = document.getElementById('batch-paste-area');
  const items = parseBatchList(input ? input.value : '');
  if (summary) summary.style.display = items.length > 0 ? 'block' : 'none';
  if (countEl) countEl.textContent = batchText('batch_parsed_count', { count: items.length });
}

async function handleBatchFileChange(e) {
  const file = e.target.files && e.target.files[0];
  if (!file) return;
  const text = await file.text();
  const pasteArea = document.getElementById('batch-paste-area');
  if (pasteArea) pasteArea.value = text;
  updateBatchParsedSummary();
}

async function handleBatchCheck(e) {
  e.preventDefault();

  if (!state.isAuthenticated) {
    showToast(batchText('batch_need_auth'), 'error');
    switchTab('account');
    return;
  }

  const pasteArea = document.getElementById('batch-paste-area');
  const items = parseBatchList(pasteArea ? pasteArea.value : '');
  if (items.length === 0) {
    showToast(batchText('batch_no_items'), 'error');
    return;
  }

  const platform = document.getElementById('batch-platform')?.value || 'iphone';
  const outputEl = document.getElementById('batch-parsed-summary');
  const btn = document.getElementById('batch-check-btn');
  if (btn) btn.disabled = true;

  try {
    const res = await fetch('/api/batch/check', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ platform, items })
    });
    const data = await res.json();
    if (!data.success) {
      showToast(data.message || 'Ошибка запуска проверки', 'error');
      return;
    }
    batchState.items = items;
    batchState.results = null;
    batchState.selected = new Set();
    batchState.addedToHistory = new Set();
    document.getElementById('batch-results-card').style.display = 'none';
    document.getElementById('batch-download-card').style.display = 'none';
    document.getElementById('batch-check-progress-card').style.display = 'block';
    document.getElementById('batch-check-progress-fill').style.width = '0%';
    document.getElementById('batch-check-progress-percent').textContent = '0%';
    pollBatchCheck(data.jobId, items);
  } catch (err) {
    showToast('Ошибка связи с сервером', 'error');
  } finally {
    if (btn) btn.disabled = false;
  }
}

function pollBatchCheck(jobId, items) {
  if (batchState.checkPoll) clearInterval(batchState.checkPoll);
  const fillEl = document.getElementById('batch-check-progress-fill');
  const percentEl = document.getElementById('batch-check-progress-percent');
  const textEl = document.getElementById('batch-check-progress-text');

  // Pre-build the status list so each line can be updated in place.
  const statusBox = document.getElementById('batch-available-list');
  if (statusBox) {
    statusBox.innerHTML = '';
    items.forEach(item => {
      const row = document.createElement('div');
      row.className = 'batch-status-row';
      row.id = `batch-row-${item.appId}`;
      row.innerHTML = `
        <span class="batch-status-icon" id="batch-status-icon-${item.appId}">⏳</span>
        <span class="batch-status-name" title="${batchEscapeHtml(item.name)}">${batchEscapeHtml(item.name)}</span>
        <code class="batch-status-id">${item.appId}</code>
        <span class="batch-status-label" id="batch-status-label-${item.appId}">…</span>
      `;
      statusBox.appendChild(row);
    });
  }

  batchState.checkPoll = setInterval(async () => {
    try {
      const res = await fetch(`/api/batch/check/status?jobId=${jobId}`);
      const job = await res.json();
      if (!job.id) {
        clearInterval(batchState.checkPoll);
        return;
      }

      const pct = Math.min(100, Math.max(0, job.progress || 0));
      if (fillEl) fillEl.style.width = `${pct}%`;
      if (percentEl) percentEl.textContent = `${pct.toFixed(1)}%`;
      if (textEl) textEl.textContent = batchText('batch_check_progress_text', {
        done: job.done || 0,
        total: job.total || 0,
        pct: job.total > 0 ? (100 / job.total).toFixed(2) : '0'
      });

      (job.items || []).forEach(item => {
        const iconEl = document.getElementById(`batch-status-icon-${item.appId}`);
        const labelEl = document.getElementById(`batch-status-label-${item.appId}`);
        if (iconEl) {
          iconEl.textContent = item.status === 'available' ? '✅' :
            item.status === 'license-required' ? '⛔' :
            item.status === 'error' ? '⚠️' : '⏳';
        }
        if (labelEl) {
          labelEl.textContent = item.status === 'available' ? `v${item.version || '?'}` :
            item.status === 'license-required' ? 'license is required' :
            item.status === 'error' ? (item.error || 'Ошибка') : '…';
        }
      });

      if (job.status === 'completed') {
        clearInterval(batchState.checkPoll);
        batchState.results = job;
        renderBatchResults(job);
      }
    } catch (err) {
      console.error('Batch check poll error:', err);
    }
  }, 500);
}

function renderBatchResults(job) {
  const resultsCard = document.getElementById('batch-results-card');
  const summaryEl = document.getElementById('batch-results-summary');
  const availableEl = document.getElementById('batch-available-list');
  const filteredBox = document.getElementById('batch-filtered-box');
  const filteredEl = document.getElementById('batch-filtered-list');
  const platform = document.getElementById('batch-platform')?.value || 'iphone';
  const outputPath = document.getElementById('batch-output-path')?.value.trim() || '';

  const available = (job.items || []).filter(i => i.status === 'available');
  const filtered = (job.items || []).filter(i => i.status !== 'available');

  if (summaryEl) {
    summaryEl.textContent = batchText('batch_results_summary', {
      available: available.length,
      total: job.total || 0,
      filtered: filtered.length
    });
  }

  if (availableEl) {
    availableEl.innerHTML = '';
    available.forEach((item, idx) => {
      const card = document.createElement('div');
      card.className = 'app-card batch-app-card';
      card.innerHTML = `
        <div class="batch-app-select">
          <label class="checkbox-label">
            <input type="checkbox" class="batch-app-checkbox" data-appid="${item.appId}" ${idx < 10 ? 'checked' : ''}>
            <span class="checkbox-custom"></span>
            <span class="checkbox-text">
              <span class="app-name">${batchEscapeHtml(item.name)}</span>
              <span class="app-artist">App ID: <code>${item.appId}</code> · v${batchEscapeHtml(item.version || '?')}</span>
            </span>
          </label>
        </div>
        <div class="app-actions">
          <details class="batch-versions-details" data-appid="${item.appId}" data-name="${batchEscapeHtml(item.name)}">
            <summary class="btn btn-outline btn-sm">🕒 ${batchText('batch_version_history')}</summary>
            <div class="batch-versions-box" id="batch-versions-${item.appId}">
              <div class="text-secondary">${batchText('batch_versions_loading')}</div>
            </div>
          </details>
        </div>
      `;
      card.querySelector('.batch-app-checkbox').addEventListener('change', ev => {
        if (ev.target.checked) {
          batchState.selected.add(item.appId);
        } else {
          batchState.selected.delete(item.appId);
        }
        // Toggling the card checkbox controls the "latest version" selection;
        // it clears any specific version picked in the history table and
        // unticks its checkbox so the two selections never conflict.
        if (batchState.selectedVersions[item.appId]) {
          delete batchState.selectedVersions[item.appId];
          document.querySelectorAll(`.batch-version-checkbox[data-appid="${item.appId}"]`).forEach(box => {
            box.checked = false;
          });
        }
      });
      if (idx < 10) batchState.selected.add(item.appId);

      const details = card.querySelector('.batch-versions-details');
      details.addEventListener('toggle', () => {
        if (details.open) loadBatchVersions(item, platform, outputPath);
      });

      availableEl.appendChild(card);
    });
    if (available.length === 0) {
      availableEl.innerHTML = `<div class="state-container card"><div class="empty-icon">😕</div><p>${batchText('batch_no_items')}</p></div>`;
    }
  }

  if (filteredBox) filteredBox.style.display = filtered.length > 0 ? 'block' : 'none';
  if (filteredEl) {
    filteredEl.innerHTML = '';
    filtered.forEach(item => {
      const row = document.createElement('div');
      row.className = 'batch-filtered-row';
      row.innerHTML = `
        <span class="batch-status-icon">${item.status === 'license-required' ? '⛔' : '⚠️'}</span>
        <span>${batchEscapeHtml(item.name)}</span>
        <code class="batch-status-id">${item.appId}</code>
        ${item.status === 'license-required'
          ? `<span class="badge badge-error">license is required</span>`
          : `<span class="badge badge-error">${batchEscapeHtml(item.error || 'Ошибка')}</span>`}
      `;
      filteredEl.appendChild(row);
    });
  }

  resultsCard.style.display = 'block';
}

// Loads version history for one app card (lazily on first expand). Uses
// app-scoped DOM ids so multiple apps can be expanded simultaneously. Unlike
// the standalone Version History tab (which has a per-row "Download" button),
// the batch variant shows a checkbox per version plus a "Download selected"
// button so several builds can be queued at once.
async function loadBatchVersions(item, platform, outputPath) {
  const container = document.getElementById(`batch-versions-${item.appId}`);
  if (!container || container.dataset.loaded === '1') return;
  container.dataset.loaded = '1';

  let ids = item.externalVersionIdentifiers || [];
  if (ids.length === 0) {
    // Fall back to the version-history endpoint when the check response did
    // not carry the build list.
    try {
      const res = await fetch(`/api/versions?appId=${item.appId}`);
      const data = await res.json();
      if (data.success && (data.externalVersionIdentifiers || []).length > 0) {
        ids = data.externalVersionIdentifiers;
      }
    } catch (err) { /* ignore */ }
  }

  if (ids.length === 0) {
    container.innerHTML = `<div class="text-secondary">${batchText('batch_no_versions')}</div>`;
    return;
  }

  const reversed = [...ids].reverse();

  const table = document.createElement('table');
  table.className = 'versions-table batch-versions-table';
  table.innerHTML = `
    <thead>
      <tr>
        <th>${batchText('version_col_build')}</th>
        <th>${batchText('version_col_display')}</th>
        <th>${batchText('version_col_date')}</th>
        <th>${batchText('version_col_select')}</th>
      </tr>
    </thead>
    <tbody></tbody>
  `;
  const tbody = table.querySelector('tbody');

  reversed.forEach((vId, idx) => {
    const row = document.createElement('tr');
    const isPicked = batchState.selectedVersions[item.appId] === vId;
    row.innerHTML = `
      <td><code>${vId}</code> ${idx === 0 ? `<span class="badge badge-success">${batchText('batch_latest_badge')}</span>` : ''}</td>
      <td id="batch-disp-${item.appId}-${vId}">…</td>
      <td id="batch-date-${item.appId}-${vId}">…</td>
      <td class="batch-version-select-cell">
        <label class="checkbox-label">
          <input type="checkbox" class="batch-version-checkbox" data-appid="${item.appId}" data-versionid="${batchEscapeHtml(vId)}" ${isPicked ? 'checked' : ''}>
          <span class="checkbox-custom"></span>
        </label>
      </td>
    `;
    const cb = row.querySelector('.batch-version-checkbox');
    cb.addEventListener('change', ev => {
      onBatchVersionPicked(item.appId, vId, ev.target.checked);
    });
    tbody.appendChild(row);
  });

  const hint = document.createElement('div');
  hint.className = 'batch-versions-hint text-secondary';
  hint.textContent = batchText('batch_version_pick_hint');

  container.innerHTML = '';
  container.appendChild(table);
  container.appendChild(hint);

  // Fill display version / release date with bounded concurrency.
  const CONCURRENCY = 5;
  let next = 0;
  async function worker() {
    while (next < reversed.length) {
      const vId = reversed[next++];
      try {
        const res = await fetch(`/api/version-metadata?appId=${item.appId}&versionId=${encodeURIComponent(vId)}`);
        const data = await res.json();
        const dispEl = document.getElementById(`batch-disp-${item.appId}-${vId}`);
        const dateEl = document.getElementById(`batch-date-${item.appId}-${vId}`);
        if (dispEl) dispEl.innerHTML = data.success ? renderDisplayVersionCell(data.displayVersion, data.minimumOSVersion) : '—';
        if (dateEl) dateEl.textContent = data.success ? (data.releaseDate || '—') : '—';
      } catch (err) {
        const dispEl = document.getElementById(`batch-disp-${item.appId}-${vId}`);
        const dateEl = document.getElementById(`batch-date-${item.appId}-${vId}`);
        if (dispEl) dispEl.textContent = '—';
        if (dateEl) dateEl.textContent = '—';
      }
    }
  }
  const workers = [];
  for (let i = 0; i < Math.min(CONCURRENCY, reversed.length); i++) {
    workers.push(worker());
  }
  await Promise.all(workers);
}

// Handles ticking/unticking a specific version in an app's version-history
// table. Picking a version means "download this build with the rest instead of
// the latest": it selects the app for the batch (and ticks the card checkbox),
// records the chosen versionId, and enforces a single pick per app. Unticking
// clears the pick so the app falls back to the latest version (card stays
// selected).
function onBatchVersionPicked(appId, versionId, checked) {
  if (checked) {
    // Only one version per app can be picked: untick any sibling checkbox.
    document.querySelectorAll(`.batch-version-checkbox[data-appid="${appId}"]`).forEach(box => {
      if (box.getAttribute('data-versionid') !== versionId) box.checked = false;
    });
    batchState.selectedVersions[appId] = versionId;
    batchState.selected.add(appId);
    // Ticking a version replaces the "latest" selection driven by the card
    // checkbox, so clear the card checkbox to signal a specific build is used.
    const cardBox = document.querySelector(`.batch-app-checkbox[data-appid="${appId}"]`);
    if (cardBox) cardBox.checked = false;
  } else {
    delete batchState.selectedVersions[appId];
    // No version picked anymore: fall back to the latest version and reflect
    // that by re-ticking the card checkbox.
    batchState.selected.add(appId);
    const cardBox = document.querySelector(`.batch-app-checkbox[data-appid="${appId}"]`);
    if (cardBox) cardBox.checked = true;
  }
}

// Saves the list of apps that passed the Apple ID check (status "available")
// back to a text file in the same "Name: AppID" format as Apps_ID_List.txt,
// so the remaining apps can be reused later.
function saveBatchList() {
  const results = batchState.results;
  if (!results) return;

  const available = (results.items || []).filter(i => i.status === 'available');
  if (available.length === 0) {
    showToast(batchText('batch_no_items'), 'error');
    return;
  }

  const content = available
    .map(i => `${i.name || i.appId}: ${i.appId}`)
    .join('\n') + '\n';

  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'apps_list.txt';
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);

  showToast(batchText('batch_save_toast'), 'success');
}

function batchSelectAll(select) {
  const boxes = document.querySelectorAll('.batch-app-checkbox');
  boxes.forEach(box => {
    box.checked = select;
    const appId = parseInt(box.getAttribute('data-appid'), 10);
    if (select) batchState.selected.add(appId);
    else batchState.selected.delete(appId);
  });
  // "Select all/none" drives the latest-version selection, so drop any specific
  // version picks and untick their checkboxes to keep the two views consistent.
  batchState.selectedVersions = {};
  document.querySelectorAll('.batch-version-checkbox').forEach(box => {
    box.checked = false;
  });
}

async function startBatchDownload() {
  if (!state.isAuthenticated) {
    showToast(batchText('batch_need_auth'), 'error');
    switchTab('account');
    return;
  }

  const results = batchState.results;
  if (!results) return;

  const available = (results.items || []).filter(i => i.status === 'available');
  const selected = available.filter(i => batchState.selected.has(i.appId));
  if (selected.length === 0) {
    showToast(batchText('batch_no_selected'), 'error');
    return;
  }

  const platform = document.getElementById('batch-platform')?.value || 'iphone';
  const outputPath = document.getElementById('batch-output-path')?.value.trim() || '';

  const btn = document.getElementById('batch-download-btn');
  if (btn) btn.disabled = true;

  try {
    const res = await fetch('/api/batch/download', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        platform,
        outputPath,
        items: selected.map(i => ({
          appId: i.appId,
          name: i.name,
          externalVersionId: batchState.selectedVersions[i.appId] || ''
        }))
      })
    });
    const data = await res.json();
    if (!data.success) {
      showToast(data.message || 'Ошибка запуска массовой загрузки', 'error');
      return;
    }
    const downloadCard = document.getElementById('batch-download-card');
    downloadCard.style.display = 'block';
    document.getElementById('batch-download-progress-fill').style.width = '0%';
    document.getElementById('batch-download-progress-percent').textContent = '0%';
    pollBatchDownload(data.batchId, selected);
    // The progress card renders at the bottom of the page, so bring it into
    // view right away — otherwise it is not obvious the download has started.
    requestAnimationFrame(() => {
      downloadCard.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });
    showToast(batchText('batch_download_started_toast'), 'success');
  } catch (err) {
    showToast('Ошибка связи с сервером', 'error');
  } finally {
    if (btn) btn.disabled = false;
  }
}

function pollBatchDownload(batchId, items) {
  if (batchState.downloadPoll) clearInterval(batchState.downloadPoll);
  const fillEl = document.getElementById('batch-download-progress-fill');
  const percentEl = document.getElementById('batch-download-progress-percent');
  const textEl = document.getElementById('batch-download-progress-text');
  const detailEl = document.getElementById('batch-download-progress-detail');
  const itemsEl = document.getElementById('batch-download-items');

  if (itemsEl) {
    itemsEl.innerHTML = '';
    items.forEach(item => {
      const row = document.createElement('div');
      row.className = 'batch-download-row';
      row.id = `batch-dl-row-${item.appId}`;
      row.innerHTML = `
        <div class="batch-download-row-head">
          <span class="batch-status-name">${batchEscapeHtml(item.name)}</span>
          <code class="batch-status-id">${item.appId}</code>
          <span class="batch-status-label" id="batch-dl-label-${item.appId}">${batchText('batch_download_status_queued')}</span>
        </div>
        <div class="dl-progress-bar-bg slim">
          <div id="batch-dl-fill-${item.appId}" class="dl-progress-bar-fill" style="width:0%"></div>
        </div>
        <div class="batch-download-row-foot">
          <span id="batch-dl-detail-${item.appId}" class="text-secondary">—</span>
          <button id="batch-dl-open-${item.appId}" class="btn btn-outline btn-sm" style="display:none" onclick="openOutputFolder('')">📂 ${batchText('open_folder_btn')}</button>
        </div>
      `;
      itemsEl.appendChild(row);
    });
  }

  batchState.downloadPoll = setInterval(async () => {
    try {
      const res = await fetch(`/api/batch/download/status?batchId=${batchId}`);
      const job = await res.json();
      if (!job.id) {
        clearInterval(batchState.downloadPoll);
        return;
      }

      const pct = Math.min(100, Math.max(0, job.progress || 0));
      if (fillEl) fillEl.style.width = `${pct}%`;
      if (percentEl) percentEl.textContent = `${pct.toFixed(1)}%`;
      if (textEl) textEl.textContent = batchText('batch_download_progress_done', {
        done: job.completedCount || 0,
        total: job.total || 0,
        errors: job.errors || 0
      });

      (job.items || []).forEach(item => {
        const labelEl = document.getElementById(`batch-dl-label-${item.appId}`);
        const fill = document.getElementById(`batch-dl-fill-${item.appId}`);
        const detail = document.getElementById(`batch-dl-detail-${item.appId}`);
        const openBtn = document.getElementById(`batch-dl-open-${item.appId}`);

        const statusKey = `batch_download_status_${item.status}`;
        if (labelEl) labelEl.textContent = i18n[state.lang]?.[statusKey] || item.status;
        if (fill) fill.style.width = `${Math.min(100, item.progress || 0)}%`;
        if (detail) {
          if (item.status === 'error') {
            detail.textContent = item.error || '';
          } else if (item.totalBytes > 0) {
            const sizePart = `${formatBytes(item.bytesRead || 0)} / ${formatBytes(item.totalBytes)}`;
            detail.textContent = item.outputPath ? `${sizePart} · ${item.outputPath}` : sizePart;
          } else {
            detail.textContent = item.outputPath || '';
          }
        }

        if (item.status === 'completed') {
          if (openBtn) {
            openBtn.style.display = 'inline-flex';
            openBtn.setAttribute('onclick', `openOutputFolder('${(item.outputPath || '').replace(/\\/g, '\\\\').replace(/'/g, "\\'")}')`);
          }
          if (!batchState.addedToHistory.has(item.appId)) {
            batchState.addedToHistory.add(item.appId);
            addToHistory({
              appName: item.name,
              bundleId: '',
              version: '',
              outputPath: item.outputPath,
              bytes: item.totalBytes || item.bytesRead || 0,
              date: new Date().toLocaleString()
            });
          }
        }

        // Feed the Downloads tab / badge via the per-app job, and drop the
        // entry once its job reaches a terminal state.
        if (item.jobId) {
          if (item.status === 'completed' || item.status === 'error') {
            state.activeDownloads.delete(item.jobId);
          } else {
            state.activeDownloads.set(item.jobId, {
              id: item.jobId,
              appName: item.name,
              bundleId: '',
              appId: item.appId,
              version: '',
              progress: item.progress || 0,
              bytesRead: item.bytesRead || 0,
              totalBytes: item.totalBytes || 0,
              speed: '',
              status: item.status,
              error: item.error || '',
              outputPath: item.outputPath || '',
              createdAt: Date.now() / 1000
            });
          }
          updateDownloadsBadge();
          renderActiveDownloadsTab();
        }
      });

      if (job.status === 'completed') {
        clearInterval(batchState.downloadPoll);
        showToast(batchText('batch_download_done_title'), 'success');
      }
    } catch (err) {
      console.error('Batch download poll error:', err);
    }
  }, 500);
}

// ==========================================
// Install .IPA to a connected iOS device
// ==========================================

const installState = {
  devices: [],
  toolsAvailable: false,
  tools: [],
  currentJobId: null
};

let installPollTimer = null;

async function refreshInstallDevices() {
  const loading = document.getElementById('install-devices-loading');
  const empty = document.getElementById('install-devices-empty');
  const list = document.getElementById('install-devices-list');
  const select = document.getElementById('install-device-select');
  const warning = document.getElementById('install-tools-warning');
  const warningText = document.getElementById('install-tools-warning-text');

  if (loading) loading.style.display = 'block';
  if (empty) empty.style.display = 'none';
  if (list) list.innerHTML = '';

  try {
    const res = await fetch('/api/install/devices');
    const data = await res.json();
    if (loading) loading.style.display = 'none';

    if (!data.success) {
      showToast(data.message || batchText('install_devices_refresh_error'), 'error');
      if (empty) empty.style.display = 'block';
      return;
    }

    const devices = data.devices || [];
    installState.devices = devices;
    installState.toolsAvailable = !!data.toolsAvailable;
    installState.tools = data.tools || [];
    renderInstallDriverStatus(data.driver);

    // Refresh the device selector while preserving the prior selection.
    const previous = select ? select.getAttribute('data-udid') || select.value : '';
    if (select) {
      select.innerHTML = '';
      const placeholder = document.createElement('option');
      placeholder.value = '';
      placeholder.textContent = batchText('install_device_select_placeholder');
      select.appendChild(placeholder);
      devices.forEach(dev => {
        const opt = document.createElement('option');
        opt.value = dev.udid;
        const label = dev.modelName || dev.name || dev.udid;
        const desc = `${label}${dev.productVersion ? ` (iOS ${dev.productVersion})` : ''}${dev.name && dev.name !== dev.modelName ? ` — ${dev.name}` : ''}`;
        opt.textContent = desc;
        select.appendChild(opt);
      });
      if (devices.some(d => d.udid === previous)) {
        select.value = previous;
        select.setAttribute('data-udid', previous);
      }
    }

    if (list) {
      devices.forEach(dev => {
        const card = document.createElement('div');
        card.className = 'install-device-card' + (select && select.value === dev.udid ? ' selected' : '');
        const label = dev.modelName || dev.name || batchText('install_device_select_placeholder');
        const nameSuffix = dev.name && dev.name !== dev.modelName ? dev.name : '';
        const version = dev.productVersion ? (dev.productVersion || '') : '';
        card.innerHTML = `
          <div class="install-device-icon">📱</div>
          <div class="install-device-meta">
            <div class="install-device-name">${batchEscapeHtml(label)}${nameSuffix ? ` <span class="install-device-model">${batchEscapeHtml(nameSuffix)}</span>` : ''}</div>
            <div class="install-device-udid">${batchEscapeHtml(dev.udid)}</div>
            <div class="install-device-detail">
              ${version ? `iOS ${batchEscapeHtml(version)}` : ''}
              ${dev.modelName && dev.modelName !== dev.name ? ` · ${batchEscapeHtml(dev.modelName)}` : ''}
            </div>
          </div>
        `;
        card.addEventListener('click', () => {
          if (select) {
            select.value = dev.udid;
            select.setAttribute('data-udid', dev.udid);
          }
          document.querySelectorAll('.install-device-card').forEach(c => c.classList.remove('selected'));
          card.classList.add('selected');
        });
        list.appendChild(card);
      });
    }

    if (devices.length === 0) {
      if (empty) empty.style.display = 'block';
    }

    if (warning && warningText) {
      if (installState.toolsAvailable) {
        warning.style.display = 'none';
      } else {
        warning.style.display = 'flex';
        warningText.textContent = batchText('install_tools_missing_desc');
      }
    }
  } catch (err) {
    if (loading) loading.style.display = 'none';
    if (empty) empty.style.display = 'block';
    showToast(batchText('install_devices_refresh_error'), 'error');
  }
}

function updateInstallFileChosen() {
  const input = document.getElementById('install-file-input');
  const text = document.getElementById('install-drop-text');
  const icon = document.getElementById('install-drop-icon');
  if (!input || !text) return;

  const file = input.files && input.files[0];
  if (file) {
    text.textContent = `${batchText('install_file_selected')} ${file.name}`;
    if (icon) icon.textContent = '✅';
  } else {
    text.textContent = batchText('install_dropzone_text');
    if (icon) icon.textContent = '📦';
  }
}

function renderInstallDriverStatus(driver) {
  const card = document.getElementById('install-driver-card');
  const iconEl = document.getElementById('install-driver-icon');
  const titleEl = document.getElementById('install-driver-title');
  const textEl = document.getElementById('install-driver-text');
  const linksEl = document.getElementById('install-driver-links');
  if (!card) return;

  const dict = i18n[state.lang] || i18n.ru;
  const links = [];

  if (!driver || driver.required === false) {
    // macOS/Linux or unknown state: Apple Mobile Device Support is not needed.
    card.style.display = 'none';
    return;
  }

  card.style.display = 'flex';
  if (iconEl) iconEl.textContent = driver.installed ? '✅' : '⚠️';
  if (titleEl) titleEl.textContent = dict.install_driver_check_title;
  if (textEl) textEl.textContent = driver.installed ? dict.install_driver_ok : dict.install_driver_missing;

  if (linksEl) {
    linksEl.innerHTML = '';
    if (!driver.installed && driver.downloadUrl) {
      const a1 = document.createElement('a');
      a1.href = driver.downloadUrl;
      a1.target = '_blank';
      a1.rel = 'noopener';
      a1.className = 'btn btn-outline btn-sm';
      a1.textContent = '⬇️ ' + dict.install_driver_download;
      linksEl.appendChild(a1);
    }
    if (!driver.installed && driver.itunesUrl) {
      const a2 = document.createElement('a');
      a2.href = driver.itunesUrl;
      a2.target = '_blank';
      a2.rel = 'noopener';
      a2.className = 'btn btn-outline btn-sm';
      a2.textContent = '🍏 ' + dict.install_driver_itunes;
      linksEl.appendChild(a2);
    }
  }
}

async function handleInstallSubmit(e) {
  e.preventDefault();

  const select = document.getElementById('install-device-select');
  const fileInput = document.getElementById('install-file-input');
  const udid = select ? select.value : '';
  const deviceName = select && select.selectedIndex >= 0 ? select.options[select.selectedIndex].textContent : '';
  const file = fileInput && fileInput.files && fileInput.files[0];

  if (!udid) {
    showToast(batchText('install_needs_device'), 'error');
    return;
  }
  if (!file) {
    showToast(batchText('install_needs_file'), 'error');
    return;
  }

  const btn = document.getElementById('install-submit-btn');
  if (btn) {
    btn.disabled = true;
    btn.textContent = '⏳ ...';
  }

  const formData = new FormData();
  formData.append('udid', udid);
  formData.append('deviceName', deviceName);
  formData.append('file', file, file.name);

  try {
    const res = await fetch('/api/install/upload', {
      method: 'POST',
      body: formData
    });
    const data = await res.json();

    if (!data.success) {
      showToast(data.message || batchText('install_error'), 'error');
      return;
    }

    installState.currentJobId = data.jobId;
    showInstallProgressCard(deviceName, file.name);
    pollInstallJob(data.jobId);
    showToast(batchText('install_started_toast'), 'success');
  } catch (err) {
    showToast(batchText('install_error'), 'error');
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = `📲 <span>${batchText('install_button')}</span>`;
    }
  }
}

function showInstallProgressCard(deviceName, fileName) {
  const card = document.getElementById('install-progress-card');
  const subtitle = document.getElementById('install-progress-subtitle');
  const fill = document.getElementById('install-progress-fill');
  const percent = document.getElementById('install-progress-percent');
  const message = document.getElementById('install-progress-message');
  const log = document.getElementById('install-progress-log');

  if (card) card.style.display = 'block';
  if (subtitle) subtitle.textContent = `${deviceName || ''} · ${fileName || ''}`.trim();
  if (fill) {
    fill.classList.remove('installing');
    fill.style.width = '0%';
  }
  if (percent) percent.textContent = '0%';
  if (message) message.textContent = batchText('install_status_queued');
  if (log) log.textContent = '';

  if (card) card.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function pollInstallJob(jobId) {
  if (installPollTimer) clearInterval(installPollTimer);

  const fill = document.getElementById('install-progress-fill');
  const percent = document.getElementById('install-progress-percent');
  const message = document.getElementById('install-progress-message');
  const log = document.getElementById('install-progress-log');
  const subtitle = document.getElementById('install-progress-subtitle');

  installPollTimer = setInterval(async () => {
    try {
      const res = await fetch(`/api/install/status?jobId=${jobId}`);
      const job = await res.json();
      if (!job.id) {
        clearInterval(installPollTimer);
        installPollTimer = null;
        return;
      }

      if (log && job.log) log.textContent = job.log;

      if (job.status === 'completed') {
        clearInterval(installPollTimer);
        installPollTimer = null;
        if (fill) {
          fill.classList.remove('installing');
          fill.style.width = '100%';
        }
        if (percent) percent.textContent = '100%';
        if (message) message.textContent = batchText('install_completed');
        if (subtitle) subtitle.textContent = `${job.deviceName || job.udid || ''} · ${job.fileName || ''}`.trim();
        showToast(batchText('install_completed'), 'success');
      } else if (job.status === 'error') {
        clearInterval(installPollTimer);
        installPollTimer = null;
        if (fill) {
          fill.classList.remove('installing');
          fill.style.width = '100%';
        }
        if (percent) percent.textContent = '—';
        if (message) message.textContent = job.error || batchText('install_error');
        showToast(job.error || batchText('install_error'), 'error');
      } else {
        if (fill) {
          fill.classList.add('installing');
          fill.style.width = '100%';
        }
        if (percent) percent.textContent = '…';
        const label = batchText(`install_status_${job.status}`) || job.status;
        if (message) message.textContent = job.message || label;
      }
    } catch (err) {
      console.error('Install poll error:', err);
    }
  }, 700);
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
  refreshActiveDownloads();

  // Search input typing listeners
  const searchInput = document.getElementById('search-input');
  const clearBtn = document.getElementById('search-clear-btn');
  if (searchInput && clearBtn) {
    searchInput.addEventListener('input', () => {
      clearBtn.style.display = searchInput.value.length > 0 ? 'block' : 'none';
    });
  }

  // Batch mass download listeners
  const batchForm = document.getElementById('batch-form');
  if (batchForm) batchForm.addEventListener('submit', handleBatchCheck);
  const batchFileInput = document.getElementById('batch-file-input');
  if (batchFileInput) batchFileInput.addEventListener('change', handleBatchFileChange);
  const batchPasteArea = document.getElementById('batch-paste-area');
  if (batchPasteArea) batchPasteArea.addEventListener('input', updateBatchParsedSummary);
});
