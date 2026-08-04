// Force fresh re-render on Firefox/Safari Back button navigation (bypass bfcache)
window.addEventListener('pageshow', (event) => {
  if (event.persisted) {
    window.location.reload();
  }
});

// Nuke HTMX's sessionStorage history cache on every page load so stale snapshots never restore
try {
  for (let i = sessionStorage.length - 1; i >= 0; i--) {
    const key = sessionStorage.key(i);
    if (key && key.startsWith('htmx-history')) {
      sessionStorage.removeItem(key);
    }
  }
} catch (e) {}

document.addEventListener('DOMContentLoaded', () => {

  // Disable HTMX client-side history caching so navigation always fetches fresh data from server
  if (window.htmx) {
    htmx.config.historyCacheSize = 0;
    htmx.config.refreshOnHistoryMiss = true;
  }

  // Escape HTML entities to prevent XSS vulnerabilities
  function escapeHTML(str) {
    if (str === null || str === undefined) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // Detect touch / mobile device without external keyboard
  if ('ontouchstart' in window || navigator.maxTouchPoints > 0 || window.matchMedia('(pointer: coarse)').matches) {
    document.documentElement.classList.add('is-touch-device');
  }

  /* ─────────────────────────────────────────────────────
     ALERT AUTO-DISMISS & MANUAL CLOSE (4 SECONDS FADE OUT)
  ───────────────────────────────────────────────────── */
  window.dismissAlert = function(el) {
    if (!el || el._isDismissing) return;
    el._isDismissing = true;
    el.classList.add('dismissing');

    setTimeout(() => {
      if (el && el.parentNode) {
        el.parentNode.removeChild(el);
      }
    }, 360);

    // Strip success and error query parameters from URL without reloading
    if (window.location.search.includes('success=') || window.location.search.includes('error=')) {
      const url = new URL(window.location.href);
      url.searchParams.delete('success');
      url.searchParams.delete('error');
      history.replaceState({}, '', url.pathname + (url.searchParams.toString() ? '?' + url.searchParams.toString() : ''));
    }
  };

  window.initAlertAutoDismiss = function() {
    const alertSelectors = [
      '.alert-banner',
      '.alert-box',
      '.alert-dismissible'
    ];

    document.querySelectorAll(alertSelectors.join(',')).forEach(banner => {
      if (banner._autoDismissInited) return;
      banner._autoDismissInited = true;

      // Add close button if not present
      if (!banner.querySelector('button')) {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.innerHTML = '✕';
        btn.className = 'text-current opacity-70 hover:opacity-100 text-sm px-2 ml-auto cursor-pointer shrink-0';
        btn.onclick = () => window.dismissAlert(banner);
        banner.appendChild(btn);
      }

      // Auto dismiss after 4 seconds (4000ms)
      setTimeout(() => {
        window.dismissAlert(banner);
      }, 4000);
    });
  };

  initAlertAutoDismiss();

  /* ─────────────────────────────────────────────────────
     HTMX HOOKS
  ───────────────────────────────────────────────────── */
  document.body.addEventListener('htmx:beforeSwap', function(evt) {
    if (evt.detail.xhr.status === 400 || evt.detail.xhr.status === 401 || evt.detail.xhr.status === 422) {
      evt.detail.shouldSwap = true;
      evt.detail.isError = false;
    }
  });

  document.body.addEventListener('htmx:afterSwap', function(evt) {
    // Re-bind expense row navigation after HTMX swaps
    document.querySelectorAll('#expenses-container > .expense-row').forEach(bindExpenseRow);
    // Re-number badges
    reNumberRows();
    // Auto-dismiss newly rendered alert banners
    window.initAlertAutoDismiss();
    // Re-init date picker if present in swapped DOM
    if (window.initDatePicker) window.initDatePicker();

    // Smooth scroll alert banner into view if target is form alert container or login error
    const target = evt.detail?.target;
    if (target && (target.id === 'form-alert-container' || target.id === 'login-error')) {
      if (target.firstElementChild) {
        target.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      }
    }

    // Update date status badge, load report figures, and manage Overwrite Mode lock state
    handleDateStatusBadgeUpdate();
  });

  /* ─────────────────────────────────────────────────────
     COMMA FORMATTER
     All inputs with class "number-input" show commas as
     the user types; the underlying value is kept clean.
  ───────────────────────────────────────────────────── */
  function formatWithCommas(n) {
    if (n === '' || n === null || n === undefined) return '';
    const num = parseInt(String(n).replace(/,/g, ''), 10);
    if (isNaN(num)) return '';
    return num.toLocaleString('en-IN');   // e.g. 1,00,000
  }

  function getRawValue(displayVal) {
    return String(displayVal).replace(/,/g, '');
  }

  function initCommaInput(el) {
    if (el._commaInited) return;
    el._commaInited = true;

    // Convert the current value on init
    if (el.value) el.value = formatWithCommas(el.value);

    el.addEventListener('input', () => {
      const raw = getRawValue(el.value);
      const pos = el.selectionStart;
      const prevLen = el.value.length;
      el.value = formatWithCommas(raw);
      // Restore cursor position roughly
      const diff = el.value.length - prevLen;
      el.setSelectionRange(Math.max(0, pos + diff), Math.max(0, pos + diff));
      triggerPreview();
    });

    el.addEventListener('focus', () => {
      el.select();
    });

  }

  function initAllCommaInputs() {
    document.querySelectorAll('.number-input').forEach(initCommaInput);
  }
  initAllCommaInputs();

  // Re-init after HTMX swaps in new content
  document.body.addEventListener('htmx:afterSwap', initAllCommaInputs);


  /* ─────────────────────────────────────────────────────
     TRIGGER PREVIEW (debounced)
  ───────────────────────────────────────────────────── */
  function triggerPreview() {
    updateCalcStrip();
  }

  function updateCalcStrip() {
    const strip = document.getElementById('calc-strip');
    if (!strip) return;

    const getNum = (id) => {
      const el = document.getElementById(id);
      if (!el) return 0;
      return parseInt(getRawValue(el.value), 10) || 0;
    };

    const totalSale   = getNum('totalSale');
    const creditSale  = getNum('creditSale');
    const bankTransfer = getNum('bankTransfer');
    const counterCash = getNum('counterCash');

    // Sum expense amounts
    let totalExpenses = 0;
    document.querySelectorAll('input[name="expenseAmount[]"], input[name="expense_amt[]"]').forEach(inp => {
      totalExpenses += parseInt(getRawValue(inp.value), 10) || 0;
    });

    const expectedCash = totalSale - creditSale - bankTransfer - totalExpenses;
    const difference   = counterCash - expectedCash;

    const fmt = (n) => n.toLocaleString('en-IN');

    const elExp  = document.getElementById('strip-expenses');
    const elExp2 = document.getElementById('strip-expected');
    const elDiff = document.getElementById('strip-diff');
    const elDiffWrap = document.getElementById('strip-diff-wrap');

    if (elExp)  elExp.textContent  = fmt(totalExpenses);
    if (elExp2) elExp2.textContent = fmt(expectedCash);

    if (elDiff && elDiffWrap) {
      if (difference === 0) {
        elDiff.textContent = '0  (Balanced)';
        elDiffWrap.className = 'ci-value text-emerald-400';
      } else if (difference < 0) {
        elDiff.textContent = `${fmt(difference)}  (Short)`;
        elDiffWrap.className = 'ci-value text-rose-400';
      } else {
        elDiff.textContent = `+${fmt(difference)}  (Surplus)`;
        elDiffWrap.className = 'ci-value text-cyan-400';
      }
    }
  }

  // Wire main inputs to live calc
  ['totalSale','creditSale','bankTransfer','counterCash'].forEach(id => {
    const el = document.getElementById(id);
    if (el) el.addEventListener('input', triggerPreview);
  });
  // Initial calc on page load (edit mode)
  updateCalcStrip();


  /* ─────────────────────────────────────────────────────
     FORM LOCK & OVERWRITE MODE CONTROLLER
  ───────────────────────────────────────────────────── */
  function toggleFormLock(lock) {
    const formInputs = document.querySelectorAll(
      '#totalSale, #creditSale, #bankTransfer, #counterCash, textarea[name="notes"], input[name="expenseDesc[]"], input[name="expenseAmount[]"], input[name="expense_desc[]"], input[name="expense_amt[]"]'
    );
    formInputs.forEach(el => {
      el.readOnly = lock;
      if (lock) {
        el.classList.add('opacity-70', 'bg-slate-900/80', 'cursor-not-allowed');
      } else {
        el.classList.remove('opacity-70', 'bg-slate-900/80', 'cursor-not-allowed');
      }
    });

    const addExpBtn = document.getElementById('add-expense-btn');
    if (addExpBtn) {
      addExpBtn.disabled = lock;
      if (lock) addExpBtn.classList.add('opacity-50', 'pointer-events-none');
      else addExpBtn.classList.remove('opacity-50', 'pointer-events-none');
    }

    document.querySelectorAll('.del-btn').forEach(btn => {
      btn.disabled = lock;
      if (lock) btn.classList.add('opacity-50', 'pointer-events-none');
      else btn.classList.remove('opacity-50', 'pointer-events-none');
    });

    const submitBtn = document.getElementById('submit-btn');
    if (submitBtn) {
      submitBtn.disabled = lock;
      if (lock) submitBtn.classList.add('opacity-50', 'cursor-not-allowed');
      else submitBtn.classList.remove('opacity-50', 'cursor-not-allowed');
    }
  }

  // ── helpers for handleDateStatusBadgeUpdate ──────────────────────────────

  function fillFormFromReport(report) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = formatWithCommas(val || 0);
    };
    setVal('totalSale',    report.total_sale);
    setVal('creditSale',   report.credit_sale);
    setVal('bankTransfer', report.bank_transfer);
    setVal('counterCash',  report.counter_cash);

    const notesEl = document.querySelector('textarea[name="notes"]');
    if (notesEl) {
      notesEl.value = report.notes || '';
      if (window.autoExpandTextarea) window.autoExpandTextarea(notesEl);
    }

    const expContainer = document.getElementById('expenses-container');
    if (expContainer && Array.isArray(report.expenses)) {
      expContainer.innerHTML = '';
      report.expenses.forEach((exp, idx) => {
        const rowHtml = `
          <div class="expense-row" data-index="${idx+1}">
            <div class="row-badge">${idx+1}</div>
            <div class="desc-input">
              <input type="text" name="expenseDesc[]" value="${escapeHTML(exp.description||'')}" class="glass-input text-xs desc-input" placeholder="Expense description">
            </div>
            <div class="amt-input">
              <input type="text" inputmode="numeric" name="expenseAmount[]" value="${formatWithCommas(exp.amount||0)}" class="glass-input number-input text-xs amt-input" placeholder="0">
            </div>
            <button type="button" class="del-btn" onclick="removeExpenseRow(this)">✕</button>
          </div>`;
        expContainer.insertAdjacentHTML('beforeend', rowHtml);
      });
      document.querySelectorAll('#expenses-container > .expense-row').forEach(bindExpenseRow);
      reNumberRows();
    }
    if (window.triggerPreview) window.triggerPreview();
  }

  function autoExpandTextarea(el) {
    if (!el) return;
    const minHeight = 56;  // ~2 lines
    const maxHeight = 160; // ~6 lines limit
    el.style.height = 'auto';
    const contentHeight = el.scrollHeight;
    if (contentHeight > maxHeight) {
      el.style.height = maxHeight + 'px';
      el.style.overflowY = 'auto';
    } else {
      el.style.height = Math.max(minHeight, contentHeight + 4) + 'px';
      el.style.overflowY = 'hidden';
    }
  }
  window.autoExpandTextarea = autoExpandTextarea;

  document.addEventListener('input', (e) => {
    if (e.target && e.target.name === 'notes') {
      autoExpandTextarea(e.target);
    }
  });

  document.addEventListener('DOMContentLoaded', () => {
    const notesEl = document.querySelector('textarea[name="notes"]');
    if (notesEl) autoExpandTextarea(notesEl);
  });

  function clearForm() {
    ['totalSale', 'creditSale', 'bankTransfer', 'counterCash'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.value = '0';
    });
    const notesEl = document.querySelector('textarea[name="notes"]');
    if (notesEl) {
      notesEl.value = '';
      autoExpandTextarea(notesEl);
    }
    const expContainer = document.getElementById('expenses-container');
    if (expContainer) { expContainer.innerHTML = ''; reNumberRows(); }
    if (window.triggerPreview) window.triggerPreview();
  }

  function resolveLockState(status, allowOverwriteCb, overwriteWrap) {
    const isEditMode = status === 'editing' || !!document.querySelector('input[name="isEditMode"]');
    if (isEditMode) {
      if (overwriteWrap) overwriteWrap.classList.add('hidden');
      return false;
    }
    if (status === 'locked' || status === 'invalid') {
      if (overwriteWrap)  overwriteWrap.classList.add('hidden');
      if (allowOverwriteCb) allowOverwriteCb.checked = false;
      return true;
    }
    if (status === 'exists') {
      if (overwriteWrap) overwriteWrap.classList.remove('hidden');
      return !(allowOverwriteCb && allowOverwriteCb.checked);
    }
    if (overwriteWrap)  overwriteWrap.classList.add('hidden');
    if (allowOverwriteCb) allowOverwriteCb.checked = false;
    return false;
  }

  function handleDateStatusBadgeUpdate() {
    const dateBadge = document.getElementById('date-status-badge');
    if (!dateBadge) return;

    const status         = dateBadge.getAttribute('data-status');
    const reportDataRaw  = dateBadge.getAttribute('data-report');
    const allowOverwriteCb = document.getElementById('allowOverwrite');
    const overwriteWrap    = document.getElementById('overwrite-toggle-wrap');

    if (reportDataRaw && (status === 'locked' || status === 'exists')) {
      try { fillFormFromReport(JSON.parse(reportDataRaw)); } catch (e) {}
    } else if (status === 'available') {
      clearForm();
    }

    toggleFormLock(resolveLockState(status, allowOverwriteCb, overwriteWrap));
    updateCalcStrip();
  }

  document.addEventListener('change', (e) => {
    if (e.target && e.target.id === 'allowOverwrite') {
      const dateBadge = document.getElementById('date-status-badge');
      const status = dateBadge ? dateBadge.getAttribute('data-status') : '';
      if (status === 'exists') {
        toggleFormLock(!e.target.checked);
      }
    }
  });
  function getPKTodayStr() {
    try {
      return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Karachi' }).format(new Date());
    } catch (e) {
      const d = new Date();
      return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`;
    }
  }

  function getPKTodayDate() {
    const s = getPKTodayStr();
    const p = s.split('-');
    return new Date(+p[0], +p[1]-1, +p[2]);
  }

  const submittedDatesCache = new Map();
  function clearSubmittedDatesCache() {
    submittedDatesCache.clear();
  }
  window.clearSubmittedDatesCache = clearSubmittedDatesCache;

  document.body.addEventListener('reportSaved', () => {
    clearSubmittedDatesCache();
    if (dpOpen) {
      dpRenderGrid();
    }
    const reportDateInput = document.getElementById('reportDate');
    if (reportDateInput && window.htmx) {
      const statusContainer = document.getElementById('date-status-container');
      if (statusContainer) {
        htmx.ajax('GET', '/reports/check-date?reportDate=' + encodeURIComponent(reportDateInput.value), { target: '#date-status-container', swap: 'innerHTML' });
      }
    }
  });
  async function fetchSubmittedDates(monthStr) {
    if (!monthStr) return { dates: new Set(), canEdit: false, userRole: '' };
    if (submittedDatesCache.has(monthStr)) return submittedDatesCache.get(monthStr);
    try {
      const res = await fetch('/reports/submitted-dates?month=' + encodeURIComponent(monthStr), {
        headers: { 'X-Requested-With': 'XMLHttpRequest' }
      });
      if (res.ok) {
        const data = await res.json();
        const info = {
          dates: new Set(data.dates || []),
          canEdit: !!data.canEdit,
          userRole: data.userRole || ''
        };
        submittedDatesCache.set(monthStr, info);
        return info;
      }
    } catch (e) {}
    const fallback = { dates: new Set(), canEdit: false, userRole: '' };
    submittedDatesCache.set(monthStr, fallback);
    return fallback;
  }
  const WEEKDAYS   = ['Su','Mo','Tu','We','Th','Fr','Sa'];
  const MONTH_LONG = ['January','February','March','April','May','June',
                      'July','August','September','October','November','December'];
  const MONTH_SHORT = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];

  let dpSelected = null;  // Date object
  let dpViewYear, dpViewMonth;
  let dpFocused = null;
  let dpOpen = false;
  let dpSpinnerOpen = false;

  // Register global outside-click & keydown handlers ONCE
  document.addEventListener('click', (e) => {
    if (!dpOpen) return;
    const curPanel   = document.getElementById('dp-panel');
    const curTrigger = document.getElementById('dp-trigger');
    if (curPanel && curPanel.contains(e.target)) return;
    if (curTrigger && (curTrigger === e.target || curTrigger.contains(e.target))) return;
    dpClose();
  });

  document.addEventListener('keydown', (e) => {
    if (dpOpen && !dpSpinnerOpen) dpHandleCalKey(e);
  });

  window.initDatePicker = function() {
    const dpTrigger = document.getElementById('dp-trigger');
    const dpPanel   = document.getElementById('dp-panel');
    const dpHidden  = document.getElementById('reportDate');

    if (!dpTrigger || !dpPanel) return;
    if (dpTrigger._dpInited) return;
    dpTrigger._dpInited = true;

    // Reset date selection state from hidden input
    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    if (dpHidden && dpHidden.value) {
      const parts = dpHidden.value.split('-');
      if (parts.length === 3) {
        dpSelected = new Date(+parts[0], +parts[1]-1, +parts[2]);
      }
    } else {
      dpSelected = yesterday;
    }
    dpSetDate(dpSelected, false);

    dpTrigger.addEventListener('click', (e) => {
      e.stopPropagation();
      dpToggle();
    });
    dpTrigger.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); dpToggle(); }
      if (e.key === 'Escape') dpClose();
      if (dpOpen) dpHandleCalKey(e);
    });

    dpPanel.addEventListener('click', (e) => {
      e.stopPropagation();
    });

    // Nav buttons
    document.getElementById('dp-prev')?.addEventListener('click', (e) => { e.stopPropagation(); dpStepMonth(-1); });
    document.getElementById('dp-next')?.addEventListener('click', (e) => { e.stopPropagation(); dpStepMonth(1); });
    document.getElementById('dp-today')?.addEventListener('click', (e) => { e.stopPropagation(); dpSetDate(new Date(), true); dpClose(); });
    document.getElementById('dp-clear')?.addEventListener('click', (e) => { e.stopPropagation(); dpSelected=null; dpUpdateTrigger(); dpClose(); });
    document.getElementById('dp-spin-open')?.addEventListener('click', (e) => { e.stopPropagation(); dpOpenSpinner(); });
    document.getElementById('dp-spin-back')?.addEventListener('click', (e) => { e.stopPropagation(); dpCloseSpinner(); });
  };

  window.initDatePicker();

  function dpToggle() {
    dpOpen ? dpClose() : dpOpenPicker();
  }

  function dpOpenPicker() {
    const dpPanel    = document.getElementById('dp-panel');
    const dpTrigger  = document.getElementById('dp-trigger');
    const dpSpinView = document.getElementById('dp-spin-view');
    const dpGridView = document.getElementById('dp-grid-view');

    if (!dpPanel) return;
    dpOpen = true;
    clearSubmittedDatesCache();
    dpPanel.classList.remove('hidden');
    dpSpinnerOpen = false;
    if (dpSpinView) dpSpinView.style.display = 'none';
    if (dpGridView) dpGridView.classList.remove('hidden');
    dpViewYear  = dpSelected ? dpSelected.getFullYear()  : new Date().getFullYear();
    dpViewMonth = dpSelected ? dpSelected.getMonth()     : new Date().getMonth();
    dpFocused   = dpSelected ? new Date(dpSelected)      : null;
    dpRenderGrid();
    if (dpTrigger) dpTrigger.setAttribute('aria-expanded', 'true');
  }

  function dpClose() {
    const dpPanel   = document.getElementById('dp-panel');
    const dpTrigger = document.getElementById('dp-trigger');

    dpOpen = false;
    if (dpPanel) dpPanel.classList.add('hidden');
    if (dpTrigger) {
      dpTrigger.setAttribute('aria-expanded', 'false');
      dpTrigger.focus();
    }
  }

  // ── helpers for dpSetDate ──────────────────────────────────────────

  function dpNotifyDateChange(newVal) {
    const statusContainer = document.getElementById('date-status-container');
    if (statusContainer && window.htmx) {
      htmx.ajax('GET', '/reports/check-date?reportDate=' + newVal, { target: '#date-status-container', swap: 'innerHTML' });
    }
  }

  // Returns true if it handled navigation (so caller skips triggerPreview)
  function dpHandleDashboardNav(newVal, oldVal) {
    const dashContainer = document.getElementById('dashboard-content');
    if (!dashContainer || !newVal || !oldVal || oldVal === newVal) return false;
    if (!window.htmx) return false;
    const url = '/?date=' + newVal + '&partial=true';
    htmx.ajax('GET', url, { target: '#dashboard-content', swap: 'innerHTML' });
    history.pushState({ path: '/?date=' + newVal }, '', '/?date=' + newVal);
    return true;
  }

  function dpSetDate(d, updateView) {
    const dpHidden = document.getElementById('reportDate');
    const oldVal   = dpHidden ? dpHidden.value : '';

    dpSelected = d ? new Date(d) : null;
    dpUpdateTrigger();
    const newVal = dpSelected ? dpIso(dpSelected) : '';

    if (dpHidden) {
      dpHidden.value = newVal;
      if (oldVal !== newVal) {
        dpHidden.dispatchEvent(new Event('change', { bubbles: true }));
        dpNotifyDateChange(newVal);
      }
    }

    if (updateView && dpSelected) {
      dpViewYear  = dpSelected.getFullYear();
      dpViewMonth = dpSelected.getMonth();
    }
    if (dpOpen) dpRenderGrid();

    if (!dpHandleDashboardNav(newVal, oldVal)) triggerPreview();
  }

  function dpUpdateTrigger() {
    const curLabel = document.getElementById('dp-label');
    if (!curLabel) return;
    if (dpSelected) {
      const d = dpSelected;
      const monthName = MONTH_SHORT[d.getMonth() % 12] || '';
      curLabel.textContent = `${String(d.getDate()).padStart(2,'0')} ${monthName} ${d.getFullYear()}`;
      curLabel.classList.remove('placeholder');
    } else {
      curLabel.textContent = 'Select date';
      curLabel.classList.add('placeholder');
    }
  }

  async function dpRenderGrid() {
    const dpGrid = document.getElementById('dp-grid');
    if (!dpGrid) return;

    // Enforce lower month bound (July 2026 = 2026-06 index)
    if (dpViewYear < 2026 || (dpViewYear === 2026 && dpViewMonth < 6)) {
      dpViewYear = 2026;
      dpViewMonth = 6;
    }

    const headerSpan = document.querySelector('#dp-spin-open > span');
    if (headerSpan) {
      const monthLongName = MONTH_LONG[dpViewMonth % 12] || '';
      headerSpan.textContent = `${monthLongName} ${dpViewYear}`;
    }

    const monthStr = `${dpViewYear}-${String(dpViewMonth+1).padStart(2,'0')}`;
    const submittedInfo = await fetchSubmittedDates(monthStr);

    const pkTodayStr = getPKTodayStr();
    const pkTodayDate = getPKTodayDate();
    const first   = new Date(dpViewYear, dpViewMonth, 1);
    const offset  = first.getDay();
    const daysInM = new Date(dpViewYear, dpViewMonth+1, 0).getDate();
    const daysInP = new Date(dpViewYear, dpViewMonth, 0).getDate();

    const cells = [];
    for (let i = offset-1; i >= 0; i--) {
      const y = dpViewMonth === 0 ? dpViewYear-1 : dpViewYear;
      const m = dpViewMonth === 0 ? 11 : dpViewMonth-1;
      cells.push({d: daysInP-i, y, m, outside: true});
    }
    for (let d = 1; d <= daysInM; d++) cells.push({d, y: dpViewYear, m: dpViewMonth, outside: false});
    let nx = 1;
    while (cells.length % 7 !== 0) {
      const y = dpViewMonth === 11 ? dpViewYear+1 : dpViewYear;
      const m = dpViewMonth === 11 ? 0 : dpViewMonth+1;
      cells.push({d: nx++, y, m, outside: true});
    }

    dpGrid.innerHTML = cells.map(c => dpBuildCell(c)).join('');

    dpGrid.querySelectorAll('.cal-day:not([disabled])').forEach(btn => {
      btn.addEventListener('click', () => {
        const y = +btn.dataset.y, m = +btn.dataset.m, d = +btn.dataset.d;
        dpSetDate(new Date(y, m, d), true);
        dpClose();
      });
    });
  }

  function dpBuildCellClasses(c, iso, dow, flags) {
    const { isBeforeMin, isFuture, isSubmitted, isDashboard } = flags;
    let cls = 'cal-day';
    if (c.outside) cls += ' outside';
    else if (dow === 0 || dow === 6) cls += ' weekend';

    if (isDashboard) {
      if (isBeforeMin || isFuture || !isSubmitted) cls += ' outside disabled no-report-disabled';
      else cls += ' has-report-selectable';
    } else {
      if (isBeforeMin || isFuture) cls += ' outside disabled future-disabled';
      else if (isSubmitted) {
        cls += submittedInfo.canEdit ? ' already-submitted-manager' : ' already-submitted-disabled disabled';
      }
    }
    return cls;
  }

  function dpBuildCellTitle(flags) {
    const { isBeforeMin, isFuture, isSubmitted, isDashboard } = flags;
    if (isDashboard) {
      if (isSubmitted)  return 'title="View EOD Report"';
      if (isFuture)     return 'title="Future Date"';
      if (isBeforeMin)  return 'title="Prior to July 2026"';
      return 'title="No report for this date"';
    }
    if (isSubmitted)    return submittedInfo.canEdit ? 'title="Report Submitted (Click to Edit)"' : 'title="Report Already Submitted"';
    if (isFuture)       return 'title="Future Date"';
    if (isBeforeMin)    return 'title="Prior to July 2026"';
    return '';
  }

  function dpBuildCell(c) {
    const cd  = new Date(c.y, c.m, c.d);
    const iso = `${c.y}-${String(c.m+1).padStart(2,'0')}-${String(c.d).padStart(2,'0')}`;
    const dow = cd.getDay();
    const isDashboard = !!document.getElementById('dashboard-content');
    const flags = {
      isBeforeMin:  iso < '2026-07-01',
      isFuture:     iso > pkTodayStr,
      isSubmitted:  submittedInfo.dates.has(iso),
      isDashboard,
    };

    let cls = dpBuildCellClasses(c, iso, dow, flags);
    if (!c.outside && dpIsSame(cd, pkTodayDate)) cls += ' today';
    if (dpSelected && dpIsSame(cd, dpSelected))  cls += ' selected';
    if (dpFocused  && dpIsSame(cd, dpFocused))   cls += ' focused';

    const isDisabled = cls.includes('disabled');
    const disAttr    = isDisabled ? 'disabled="disabled"' : '';
    const titleAttr  = dpBuildCellTitle(flags);

    return `<button type="button" class="${escapeHTML(cls)}" data-y="${Number(c.y)}" data-m="${Number(c.m)}" data-d="${Number(c.d)}" ${disAttr} ${titleAttr}>${Number(c.d)}</button>`;
  }

  function dpStepMonth(delta) {
    let nextMonth = dpViewMonth + delta;
    let nextYear = dpViewYear;
    if (nextMonth < 0)  { nextMonth = 11; nextYear--; }
    if (nextMonth > 11) { nextMonth = 0;  nextYear++; }

    // Enforce July 2026 lower boundary
    if (nextYear < 2026 || (nextYear === 2026 && nextMonth < 6)) {
      return;
    }

    dpViewYear = nextYear;
    dpViewMonth = nextMonth;
    dpRenderGrid();
  }

  // ── helpers for dpHandleCalKey ────────────────────────────────────

  const DP_ARROW_DELTA = { ArrowRight: 1, ArrowLeft: -1, ArrowDown: 7, ArrowUp: -7 };

  // Returns true if the key was a special action (caller should return immediately)
  function dpHandleCalSpecialKey(e) {
    if (e.key === 'PageDown') { e.preventDefault(); dpStepMonth(1);  return true; }
    if (e.key === 'PageUp')   { e.preventDefault(); dpStepMonth(-1); return true; }
    if (e.key === 'Enter')    { e.preventDefault(); if (dpFocused) { dpSetDate(dpFocused, true); dpClose(); } return true; }
    if (e.key === 'Escape')   { dpClose(); return true; }
    if (e.key === 't' || e.key === 'T') { dpSetDate(new Date(), true); dpClose(); return true; }
    if (e.key === 'm' || e.key === 'M') { dpOpenSpinner(); return true; }
    return false;
  }

  function dpHandleCalKey(e) {
    if (!dpFocused && dpSelected) dpFocused = new Date(dpSelected);
    if (!dpFocused) dpFocused = new Date(dpViewYear, dpViewMonth, 1);

    if (dpHandleCalSpecialKey(e)) return;

    const delta = DP_ARROW_DELTA[e.key];
    if (delta === undefined) return;
    e.preventDefault();
    dpFocused.setDate(dpFocused.getDate() + delta);
    dpViewYear  = dpFocused.getFullYear();
    dpViewMonth = dpFocused.getMonth();
    dpRenderGrid();
  }

  // ── Spinner (month/year wheel) ──
  const ITEM_H = 32;
  const MIN_YEAR = 2026;
  const CUR_YEAR = new Date().getFullYear();
  const YEAR_LIST = [];
  for (let y = MIN_YEAR; y <= Math.max(MIN_YEAR, CUR_YEAR + 5); y++) YEAR_LIST.push(y);

  let spMonthIdx = 0, spYearIdx = 0;

  function dpOpenSpinner() {
    const dpGridView = document.getElementById('dp-grid-view');
    const dpSpinView = document.getElementById('dp-spin-view');
    dpSpinnerOpen = true;
    if (dpGridView)  dpGridView.classList.add('hidden');
    if (dpSpinView)  { dpSpinView.style.display = 'flex'; dpSpinView.style.flexDirection = 'column'; }

    const mCol = document.getElementById('dp-month-col');
    const yCol = document.getElementById('dp-year-col');
    if (!mCol || !yCol) return;

    if (!mCol._built) {
      mCol.innerHTML = '<div class="spinner-highlight"></div><div id="dp-month-track"></div>';
      document.getElementById('dp-month-track').innerHTML =
        MONTH_SHORT.map((m,i)=>`<div class="spinner-item" data-index="${Number(i)}">${escapeHTML(m)}</div>`).join('');
      wireSpinner(mCol, MONTH_SHORT.length, (idx) => { spMonthIdx = idx; });
      mCol._built = true;
    }
    if (!yCol._built) {
      yCol.innerHTML = '<div class="spinner-highlight"></div><div id="dp-year-track"></div>';
      document.getElementById('dp-year-track').innerHTML =
        YEAR_LIST.map((y,i)=>`<div class="spinner-item" data-index="${Number(i)}">${escapeHTML(String(y))}</div>`).join('');
      wireSpinner(yCol, YEAR_LIST.length, (idx) => { spYearIdx = idx; });
      yCol._built = true;
    }

    const initM = dpViewMonth;
    const initY = YEAR_LIST.indexOf(dpViewYear);
    scrollSpinner(mCol, initM);
    scrollSpinner(yCol, initY >= 0 ? initY : YEAR_LIST.indexOf(CUR_YEAR));
  }

  function dpCloseSpinner() {
    const dpGridView = document.getElementById('dp-grid-view');
    const dpSpinView = document.getElementById('dp-spin-view');
    dpSpinnerOpen = false;
    if (dpSpinView)  dpSpinView.style.display = 'none';
    if (dpGridView)  dpGridView.classList.remove('hidden');
    dpViewMonth = spMonthIdx;
    dpViewYear  = YEAR_LIST[spYearIdx] ?? dpViewYear;
    dpRenderGrid();
  }

  function wireSpinner(col, count, onIdx) {
    let dragging = false, startY = 0, startScroll = 0, lastIdx = -1;

    function curIdx() { return Math.max(0, Math.min(count-1, Math.round(col.scrollTop/ITEM_H))); }
    function report() {
      const idx = curIdx();
      if (idx !== lastIdx) {
        lastIdx = idx;
        col.querySelectorAll('.spinner-item').forEach(el => el.classList.remove('active'));
        col.querySelector(`.spinner-item[data-index="${idx}"]`)?.classList.add('active');
        onIdx(idx);
      }
    }

    col.addEventListener('scroll', () => requestAnimationFrame(report), {passive:true});
    col.addEventListener('mousedown', e => { dragging=true; startY=e.clientY; startScroll=col.scrollTop; col.classList.add('grabbing'); e.preventDefault(); });
    window.addEventListener('mousemove', e => { if(dragging) col.scrollTop = startScroll-(e.clientY-startY); });
    window.addEventListener('mouseup', () => { if(!dragging) return; dragging=false; col.classList.remove('grabbing'); col.scrollTo({top:curIdx()*ITEM_H,behavior:'smooth'}); });
    col.addEventListener('click', e => {
      const item = e.target.closest('.spinner-item');
      if (item) col.scrollTo({top:+item.dataset.index*ITEM_H, behavior:'smooth'});
    });
  }

  function scrollSpinner(col, idx) {
    col.scrollTop = idx * ITEM_H;
    requestAnimationFrame(() => {
      col.querySelectorAll('.spinner-item').forEach(el => el.classList.remove('active'));
      col.querySelector(`.spinner-item[data-index="${idx}"]`)?.classList.add('active');
    });
  }

  function dpIso(d) {
    const y = d.getFullYear();
    const m = String(d.getMonth()+1).padStart(2,'0');
    const dd = String(d.getDate()).padStart(2,'0');
    return `${y}-${m}-${dd}`;
  }
  function dpIsSame(a,b) {
    return a && b && a.getFullYear()===b.getFullYear() && a.getMonth()===b.getMonth() && a.getDate()===b.getDate();
  }


  /* ─────────────────────────────────────────────────────
     EXPENSE ROWS
  ───────────────────────────────────────────────────── */
  let rowCount = 0;

  function reNumberRows() {
    document.querySelectorAll('#expenses-container > .expense-row').forEach((row, i) => {
      const badge = row.querySelector('.row-badge');
      if (badge) badge.textContent = i + 1;
    });
    rowCount = document.querySelectorAll('#expenses-container > .expense-row').length;
  }

  function bindExpenseRow(rowEl) {
    if (rowEl._bound) return;
    rowEl._bound = true;

    const descInput = rowEl.querySelector('input[name="expenseDesc[]"]');
    const amtInput  = rowEl.querySelector('input[name="expenseAmount[]"]');
    const delBtn    = rowEl.querySelector('.del-btn');
    const badge     = rowEl.querySelector('.row-badge');

    // Init comma format on amount
    if (amtInput) initCommaInput(amtInput);

    // Badge: click to focus desc, shift-click to delete
    if (badge) {
      badge.addEventListener('click', e => {
        if (e.shiftKey) { rowEl.remove(); reNumberRows(); triggerPreview(); }
        else { descInput?.focus(); }
      });
    }

    if (delBtn) delBtn.addEventListener('click', () => {
      rowEl.remove();
      reNumberRows();
      triggerPreview();
    });

    if (amtInput) {
      amtInput.addEventListener('input', triggerPreview);
      amtInput.addEventListener('keydown', e => {
        if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey) {
          e.preventDefault();
          addExpenseRow();
        } else if (e.key === 'Backspace' && e.shiftKey) {
          e.preventDefault();
          rowEl.remove(); reNumberRows(); triggerPreview();
        } else {
          focusNeighbourInput(e, rowEl, 'input[name="expenseAmount[]"]');
        }
      });
    }

    if (descInput) {
      descInput.addEventListener('keydown', e => {
        if (e.key === 'Backspace' && e.shiftKey) {
          e.preventDefault();
          rowEl.remove(); reNumberRows(); triggerPreview();
        } else {
          focusNeighbourInput(e, rowEl, 'input[name="expenseDesc[]"]');
        }
      });
    }
  }

  function focusNeighbourInput(e, rowEl, selector) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      const next = rowEl.nextElementSibling?.querySelector(selector);
      if (next) next.focus();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      const prev = rowEl.previousElementSibling?.querySelector(selector);
      if (prev) prev.focus();
    }
  }

  window.addExpenseRow = function(descVal='', amtVal='') {
    const container = document.getElementById('expenses-container');
    if (!container) return;

    rowCount = container.querySelectorAll('.expense-row').length + 1;
    const row = document.createElement('div');
    row.className = 'expense-row';
    row.innerHTML = `
      <span class="row-badge" title="Click to focus · Shift-click to delete">${Number(rowCount)}</span>
      <div class="desc-input">
        <input type="text" name="expenseDesc[]" value="${escapeHTML(descVal)}"
               placeholder="Description" class="glass-input text-xs" autocomplete="off">
      </div>
      <div class="amt-input">
        <input type="text" inputmode="numeric" name="expenseAmount[]" value="${escapeHTML(amtVal)}"
               placeholder="0" class="glass-input number-input text-xs">
      </div>
      <button type="button" class="del-btn" title="Remove row">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    `;
    container.appendChild(row);
    bindExpenseRow(row);
    row.querySelector('input[name="expenseDesc[]"]')?.focus();
    reNumberRows();
  };

  // Bind existing rows (edit mode)
  document.querySelectorAll('#expenses-container > .expense-row').forEach(bindExpenseRow);
  reNumberRows();

  // Delegated click handler for #add-expense-btn (works seamlessly across SPA transitions & swaps)
  document.body.addEventListener('click', (e) => {
    const addBtn = e.target.closest('#add-expense-btn');
    if (addBtn) {
      e.preventDefault();
      addExpenseRow();
    }
  });


  /* ─────────────────────────────────────────────────────
     GLOBAL KEYBOARD SHORTCUTS
  ───────────────────────────────────────────────────── */
  // ── helpers for the global keydown handler ──────────────────────────

  const ALT_KEY_MAP = { d:'dp-trigger', s:'totalSale', c:'creditSale', b:'bankTransfer', k:'counterCash', a:null };

  function handleAltKeyShortcut(e) {
    const k = e.key.toLowerCase();
    if (!Object.prototype.hasOwnProperty.call(ALT_KEY_MAP, k)) return;
    e.preventDefault();
    if (k === 'a') { addExpenseRow(); return; }
    const targetId = ALT_KEY_MAP[k];
    if (targetId) document.getElementById(targetId)?.focus();
  }

  function handleDashboardArrowNav(e) {
    const prevLink = document.getElementById('dash-prev-day');
    const nextLink = document.getElementById('dash-next-day');
    if (!prevLink || !nextLink) return;
    if (e.key === 'ArrowLeft')  { e.preventDefault(); prevLink.click(); }
    if (e.key === 'ArrowRight') { e.preventDefault(); nextLink.click(); }
  }

  document.addEventListener('keydown', e => {
    const tag = document.activeElement?.tagName;
    const isTyping = ['INPUT','TEXTAREA','SELECT'].includes(tag);

    // Ctrl/Cmd+Enter → submit form
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      const form = document.querySelector('form[hx-post="/reports"]');
      if (form) { e.preventDefault(); htmx.trigger(form, 'submit'); }
      return;
    }

    // Escape → close modals / date picker
    if (e.key === 'Escape') { if (dpOpen) { dpClose(); return; } closeModal(); return; }

    // ? → shortcuts modal (only when not typing)
    if (e.key === '?' && !isTyping) { e.preventDefault(); toggleShortcutsModal(); return; }

    // Alt shortcuts work EVEN when a field is focused
    if (e.altKey) { handleAltKeyShortcut(e); return; }

    if (isTyping) return;

    // Dashboard: left/right arrow to navigate dates
    handleDashboardArrowNav(e);
  });


  /* ─────────────────────────────────────────────────────
     MODAL / OVERLAYS
  ───────────────────────────────────────────────────── */
  window.closeModal = function() {
    const reportModal = document.getElementById('report-modal-container');
    if (reportModal) reportModal.innerHTML = '';
    document.getElementById('global-shortcuts-modal')?.remove();
    document.querySelectorAll('.modal-backdrop, .modal-overlay').forEach(m => m.remove());
  };

  window.toggleShortcutsModal = function() {
    const existing = document.getElementById('global-shortcuts-modal');
    if (existing) { existing.remove(); return; }

    const modal = document.createElement('div');
    modal.id = 'global-shortcuts-modal';
    modal.className = 'fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm';
    modal.innerHTML = `
      <div class="glass-card w-full max-w-md p-6 bg-slate-950 border border-slate-800 rounded-lg shadow-2xl space-y-4">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h2 class="text-sm font-bold text-white">Keyboard Shortcuts</h2>
          <button onclick="closeModal()" class="text-slate-400 hover:text-white text-xs">✕</button>
        </div>
        <div class="space-y-0 text-xs divide-y divide-slate-800/60">
          <div class="grid grid-cols-2 gap-x-4 py-1 font-bold text-[10px] uppercase text-slate-500 tracking-wider"><span>Action</span><span>Key</span></div>
          ${[
            ['Focus Total Sales',        'Alt + S',      'text-slate-300'],
            ['Focus Credit Card Sales',  'Alt + C',      'text-slate-300'],
            ['Focus Bank Transfer',      'Alt + B',      'text-slate-300'],
            ['Focus Counter Cash',       'Alt + K',      'text-slate-300'],
            ['Focus Date Picker',        'Alt + D',      'text-slate-300'],
            ['Add Expense Row',          'Alt + A',      'text-amber-300'],
            ['New row (in Amount field)','Enter',        'text-amber-300'],
            ['Delete focused row',       'Shift+Backspace','text-rose-300'],
            ['Navigate rows',            '↑ / ↓',        'text-slate-300'],
            ['Submit form',              'Ctrl + Enter', 'text-cyan-300'],
            ['Prev/Next day (dashboard)','← / →',        'text-slate-300'],
            ['Close / Escape',           'Esc',          'text-slate-300'],
            ['This guide',               '?',            'text-slate-300'],
          ].map(([action,key,klass])=>`
            <div class="flex items-center justify-between py-1.5 gap-2">
              <span class="text-slate-400">${escapeHTML(action)}</span>
              <kbd class="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded ${escapeHTML(klass)} font-mono text-[10px] shrink-0">${escapeHTML(key)}</kbd>
            </div>
          `).join('')}
        </div>
        <div class="pt-2 text-center border-t border-slate-800">
          <button onclick="closeModal()" class="btn btn-primary text-xs py-1.5 px-4">Got it</button>
        </div>
      </div>
    `;
    document.body.appendChild(modal);
    modal.addEventListener('click', e => { if (e.target === modal) closeModal(); });
  };


  /* ─────────────────────────────────────────────────────
     AUTO-DISMISS ERROR BANNERS ON INTERACTION / TYPING
  ───────────────────────────────────────────────────── */
  window.clearFormErrors = function() {
    const loginError = document.getElementById('login-error');
    if (loginError && loginError.children.length > 0) {
      loginError.replaceChildren();
    }
    const formAlert = document.getElementById('form-alert-container');
    if (formAlert && formAlert.children.length > 0) {
      formAlert.replaceChildren();
    }
  };

  document.addEventListener('input', (e) => {
    // Only clear form alert when user is actively editing form text/number input fields
    if (e.target.closest('form') && (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA')) {
      window.clearFormErrors();
    }
  });


  // Theme is handled by the inline script in layout.html (3-way: system/dark/light).
  // Do not re-define applyTheme/toggleTheme here to avoid overwriting the correct implementation.


  /* ─────────────────────────────────────────────────────
     CLIPBOARD HELPERS
  ───────────────────────────────────────────────────── */
  window.copyTextToClipboard = function(text) {
    const doIt = () => {
      if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).catch(fallback);
      } else { fallback(); }
    };
    function fallback() {
      const ta = Object.assign(document.createElement('textarea'), {value:text, style:'position:fixed;top:-9999px;left:-9999px'});
      document.body.appendChild(ta); ta.focus(); ta.select();
      try { if (!document.execCommand('copy')) prompt('Copy:', text); }
      catch { prompt('Copy:', text); }
      document.body.removeChild(ta);
    }
    doIt();
  };

  window.copyExpensesForExcel = function() {
    const items = document.querySelectorAll('.dash-expense-item');
    if (!items.length) { alert('No expenses to copy.'); return; }
    let tsv = 'Description\tAmount\n';
    items.forEach(el => {
      const desc = el.dataset.desc || el.querySelector('.dash-exp-desc')?.innerText.trim();
      const amt  = el.dataset.amount || el.querySelector('.dash-exp-amt')?.innerText.replace(/[^0-9.-]/g,'');
      if (desc && amt) tsv += `${desc}\t${amt}\n`;
    });
    copyTextToClipboard(tsv, '✓ copied');
  };


  /* ─────────────────────────────────────────────────────
     RESIZABLE DASHBOARD PANELS (pinned right, stored in localStorage)
  ───────────────────────────────────────────────────── */
  function restoreSavedSideWidth() {
    const mainPanel = document.getElementById('dash-main-panel');
    const sidePanel = document.getElementById('dash-side-panel');
    if (!sidePanel || !mainPanel) return;
    try {
      const saved = localStorage.getItem('wedrink_side_panel_width');
      if (saved) {
        const w = parseInt(saved, 10);
        if (w >= 220 && w <= 800) {
          mainPanel.style.flex  = '1 1 0';
          mainPanel.style.width = '';
          sidePanel.style.flex  = '0 0 auto';
          sidePanel.style.width = w + 'px';
        }
      } else {
        mainPanel.style.flex = '';
        mainPanel.style.width = '';
        sidePanel.style.flex = '';
        sidePanel.style.width = '';
      }
    } catch (e) {}
  }

  function initDashboardResize() {
    const handle    = document.getElementById('dash-resize-handle');
    const mainPanel = document.getElementById('dash-main-panel');
    const sidePanel = document.getElementById('dash-side-panel');

    if (!handle || !sidePanel || !mainPanel) return;

    restoreSavedSideWidth();

    if (handle._resizeInited) return;
    handle._resizeInited = true;

    let dragging = false, startX = 0, startSide = 0;

    handle.addEventListener('mousedown', e => {
      dragging  = true;
      startX    = e.clientX;
      startSide = sidePanel.getBoundingClientRect().width;
      handle.classList.add('dragging');
      document.body.style.userSelect = 'none';
      document.body.style.cursor = 'col-resize';
    });

    window.addEventListener('mousemove', e => {
      if (!dragging) return;
      const dx = e.clientX - startX;
      const container = handle.closest('.dash-panels');
      const containerWidth = container ? container.getBoundingClientRect().width : 1200;
      const maxSide = Math.max(220, containerWidth - 300);
      const newSide = Math.max(220, Math.min(maxSide, Math.round(startSide - dx)));

      mainPanel.style.flex  = '1 1 0';
      mainPanel.style.width = '';
      sidePanel.style.flex  = '0 0 auto';
      sidePanel.style.width = newSide + 'px';

      try {
        localStorage.setItem('wedrink_side_panel_width', newSide);
      } catch (e) {}
    });

    window.addEventListener('mouseup', () => {
      if (!dragging) return;
      dragging = false;
      handle.classList.remove('dragging');
      document.body.style.userSelect = '';
      document.body.style.cursor = '';
    });

    handle.addEventListener('dblclick', () => {
      try { localStorage.removeItem('wedrink_side_panel_width'); } catch (e) {}
      restoreSavedSideWidth();
    });
  }

  initDashboardResize();
  document.body.addEventListener('htmx:afterSwap', initDashboardResize);
  document.body.addEventListener('htmx:afterSettle', restoreSavedSideWidth);


  /* ─────────────────────────────────────────────────────
     REPORTS TABLE — select all & unified export
  ───────────────────────────────────────────────────── */
  window.selectAllReports = function(masterCb) {
    document.querySelectorAll('.report-select-checkbox').forEach(cb => { cb.checked = masterCb.checked; });
  };

  window.triggerUnifiedExport = function() {
    const selected = Array.from(document.querySelectorAll('.report-select-checkbox:checked')).map(cb => cb.value);
    let url = '/export/excel';

    if (selected.length > 0) {
      url += '?ids=' + encodeURIComponent(selected.join(','));
    } else {
      const form = document.querySelector('form[hx-get="/reports"]');
      if (form) {
        const formData = new FormData(form);
        const params = new URLSearchParams();
        params.set('type', 'all');
        for (const [key, value] of formData.entries()) {
          if (value && key !== 'partial') {
            params.set(key, value);
          }
        }
        url += '?' + params.toString();
      } else {
        url += '?type=all';
      }
    }

    const a = document.createElement('a');
    a.href = url;
    a.download = 'wedrink_eod_report.xlsx';
    a.target = '_blank';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  window.exportSelectedRows = function(format) {
    window.triggerUnifiedExport();
  };

  /* ─────────────────────────────────────────────────────
     INSTANT MOBILE TOUCH SWIPE & TAB NAVIGATION (WhatsApp style)
  ───────────────────────────────────────────────────── */
  (function initTouchSwipeNavigation() {
    const pageCache = new Map();

    // Cache initial page main content
    const mainEl = document.getElementById('app-main-content') || document.querySelector('main');
    if (mainEl) {
      pageCache.set(window.location.pathname, mainEl.innerHTML);
    }

    // Background pre-fetch of tab pages for instant (0ms) switching
    function prefetchRoutes() {
      const navLinks = document.querySelectorAll('#mobile-nav-bar a.mobile-nav-item, nav a[href]');
      navLinks.forEach(link => {
        const href = link.getAttribute('data-nav-href') || link.getAttribute('href');
        if (href && (href === '/' || href === '/submit' || href === '/reports' || href === '/admin/users' || href === '/profile') && !pageCache.has(href)) {
          fetch(href, { headers: { 'X-Requested-With': 'XMLHttpRequest' } })
            .then(res => res.text())
            .then(html => {
              const parser = new DOMParser();
              const doc = parser.parseFromString(html, 'text/html');
              const fetchedMain = doc.querySelector('#app-main-content') || doc.querySelector('main');
              if (fetchedMain) {
                pageCache.set(href, fetchedMain.innerHTML);
              }
            })
            .catch(() => {});
        }
      });
    }

    if ('requestIdleCallback' in window) {
      requestIdleCallback(prefetchRoutes);
    } else {
      setTimeout(prefetchRoutes, 200);
    }

    // Instant Page Swap Function (0ms response)
    window.navigateToTabInstant = function(targetRoute, direction = 'left') {
      const currentPath = window.location.pathname;
      if (currentPath === targetRoute) return;

      const mainContainer = document.getElementById('app-main-content') || document.querySelector('main');
      if (!mainContainer) {
        window.location.href = targetRoute;
        return;
      }

      updateActiveNavLinks(targetRoute);

      const outClass = direction === 'left' ? 'slide-out-left' : 'slide-out-right';
      const inClass = direction === 'left' ? 'slide-in-right' : 'slide-in-left';

      const renderContent = (newHTML) => {
        mainContainer.className = mainContainer.className.replace(/slide-\S+/g, '').trim();
        mainContainer.classList.add(outClass);

        setTimeout(() => {
          mainContainer.innerHTML = newHTML;
          pageCache.set(targetRoute, newHTML);
          history.pushState({ path: targetRoute }, '', targetRoute);

          // Re-bind HTMX & component listeners
          if (window.htmx) htmx.process(mainContainer);
          if (window.initAlertAutoDismiss) window.initAlertAutoDismiss();
          document.body.dispatchEvent(new Event('htmx:afterSwap'));
          window.scrollTo({ top: 0, behavior: 'instant' });

          mainContainer.classList.remove(outClass);
          mainContainer.classList.add(inClass);

          setTimeout(() => {
            mainContainer.classList.remove(inClass);
          }, 120);
        }, 60);
      };

      if (pageCache.has(targetRoute)) {
        renderContent(pageCache.get(targetRoute));
      } else {
        fetch(targetRoute)
          .then(res => res.text())
          .then(html => {
            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');
            const fetchedMain = doc.querySelector('#app-main-content') || doc.querySelector('main');
            if (fetchedMain) {
              renderContent(fetchedMain.innerHTML);
            } else {
              window.location.href = targetRoute;
            }
          })
          .catch(() => {
            window.location.href = targetRoute;
          });
      }
    };

    function updateActiveNavLinks(targetRoute) {
      document.querySelectorAll('nav a[href]').forEach(link => {
        const href = link.getAttribute('href');
        const isActive = href === targetRoute;
        if (isActive) {
          link.classList.add('bg-[#e50811]', 'text-white');
          link.classList.remove('bg-[#007C77]', 'text-slate-400', 'hover:text-slate-200');
        } else {
          link.classList.remove('bg-[#e50811]', 'bg-[#007C77]', 'text-white');
          link.classList.add('text-slate-400');
        }
      });

      document.querySelectorAll('#mobile-nav-bar a.mobile-nav-item').forEach(link => {
        const href = link.getAttribute('data-nav-href') || link.getAttribute('href');
        const isActive = href === targetRoute;
        if (isActive) {
          link.classList.add('text-[#e50811]', 'bg-[#131f3a]');
          link.classList.remove('text-[#00b4d8]', 'text-slate-400');
        } else {
          link.classList.remove('text-[#e50811]', 'text-[#00b4d8]', 'bg-[#131f3a]');
          link.classList.add('text-slate-400');
        }
      });
    }

    // Sync active nav links on initial load
    updateActiveNavLinks(window.location.pathname);

    // Intercept clicks on mobile & header nav links for instant SPA switching
    document.addEventListener('click', (e) => {
      const navItem = e.target.closest('#mobile-nav-bar a.mobile-nav-item, nav a[href]');
      if (!navItem) return;
      const href = navItem.getAttribute('data-nav-href') || navItem.getAttribute('href');
      if (href && (href === '/' || href === '/submit' || href === '/reports' || href === '/admin/users' || href === '/profile')) {
        e.preventDefault();
        const currentPath = window.location.pathname;
        const routes = ['/', '/submit', '/reports', '/admin/users', '/profile'];
        const fromIdx = routes.indexOf(currentPath);
        const toIdx = routes.indexOf(href);
        const dir = toIdx >= fromIdx ? 'left' : 'right';
        window.navigateToTabInstant(href, dir);
      }
    });

    // Support browser Back / Forward buttons
    window.addEventListener('popstate', (e) => {
      const path = window.location.pathname;
      if (pageCache.has(path)) {
        window.navigateToTabInstant(path, 'right');
      } else {
        window.location.reload();
      }
    });

    // Touch Swipe Gesture Handling
    let touchStartX = 0;
    let touchStartY = 0;
    let touchStartTime = 0;

    // Returns true if el is a hardcoded scroll-exempt container
    function isScrollExemptElement(el) {
      return el.classList && (
        el.classList.contains('overflow-x-auto') ||
        el.classList.contains('date-panel') ||
        el.id === 'dp-panel' ||
        el.id === 'global-shortcuts-modal' ||
        el.id === 'report-modal-container'
      );
    }

    function isInsideHorizontallyScrollable(el) {
      let curr = el;
      while (curr && curr !== document.body && curr !== document.documentElement) {
        if (curr.nodeType === 1) {
          const style    = window.getComputedStyle(curr);
          const overflow = style.overflowX;
          if ((overflow === 'auto' || overflow === 'scroll') && curr.scrollWidth > curr.clientWidth) return true;
          if (isScrollExemptElement(curr)) return true;
        }
        curr = curr.parentElement;
      }
      return false;
    }

    document.addEventListener('touchstart', (e) => {
      if (e.touches.length !== 1) return;
      const touch = e.touches[0];
      touchStartX = touch.clientX;
      touchStartY = touch.clientY;
      touchStartTime = Date.now();
    }, { passive: true });

    // ── helpers for touchend swipe navigation ───────────────────────────
    const SWIPE_VALID_ROUTES = ['/', '/submit', '/reports', '/admin/users', '/profile'];

    function isValidSwipe(deltaX, deltaY, duration) {
      if (duration > 600) return false;
      if (Math.abs(deltaX) < 45) return false;
      if (Math.abs(deltaX) < 1.3 * Math.abs(deltaY)) return false;
      return true;
    }

    function collectSwipeRoutes(navLinks) {
      const routes = [];
      navLinks.forEach(link => {
        const href = link.getAttribute('data-nav-href') || link.getAttribute('href');
        if (href && !routes.includes(href) && SWIPE_VALID_ROUTES.includes(href)) routes.push(href);
      });
      return routes;
    }

    function resolveSwipeCurrentIndex(routes, pathname) {
      const idx = routes.indexOf(pathname);
      if (idx !== -1) return idx;
      if (pathname === '' || pathname === '/')       return routes.indexOf('/');
      if (pathname.startsWith('/submit'))            return routes.indexOf('/submit');
      if (pathname.startsWith('/reports'))           return routes.indexOf('/reports');
      if (pathname.startsWith('/admin/users'))       return routes.indexOf('/admin/users');
      if (pathname.startsWith('/profile'))           return routes.indexOf('/profile');
      return -1;
    }

    function computeSwipeTarget(routes, currentIndex, deltaX) {
      if (deltaX < 0 && currentIndex < routes.length - 1) return { index: currentIndex + 1, dir: 'left' };
      if (deltaX > 0 && currentIndex > 0)                 return { index: currentIndex - 1, dir: 'right' };
      return null;
    }

    document.addEventListener('touchend', (e) => {
      if (e.changedTouches.length !== 1) return;

      const touch    = e.changedTouches[0];
      const deltaX   = touch.clientX - touchStartX;
      const deltaY   = touch.clientY - touchStartY;
      const duration = Date.now() - touchStartTime;

      if (!isValidSwipe(deltaX, deltaY, duration)) return;

      const target = document.elementFromPoint(touch.clientX, touch.clientY) || e.target;
      if (isInsideHorizontallyScrollable(target)) return;

      const navLinks = Array.from(document.querySelectorAll('#mobile-nav-bar a.mobile-nav-item, nav a[href]'));
      if (navLinks.length === 0) return;

      const routes       = collectSwipeRoutes(navLinks);
      if (routes.length < 2) return;

      const currentIndex = resolveSwipeCurrentIndex(routes, window.location.pathname);
      if (currentIndex === -1) return;

      const swipe = computeSwipeTarget(routes, currentIndex, deltaX);
      if (swipe) window.navigateToTabInstant(routes[swipe.index], swipe.dir);
    }, { passive: true });
  })();

});
