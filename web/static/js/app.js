/* ═══════════════════════════════════════════════════════
   app.js  —  Wedrink EOD Portal
   ═══════════════════════════════════════════════════════ */

document.addEventListener('DOMContentLoaded', () => {

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
  let previewTimer;
  function triggerPreview() {
    clearTimeout(previewTimer);
    previewTimer = setTimeout(() => {
      // Update the sticky calc strip if it exists (client-side, no round-trip needed)
      updateCalcStrip();
    }, 80);
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
    document.querySelectorAll('input[name="expenseAmount[]"]').forEach(inp => {
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
     CUSTOM DATE PICKER
  /* ─────────────────────────────────────────────────────
     CUSTOM ACCESSIBLE DATE PICKER
  ───────────────────────────────────────────────────── */
  const WEEKDAYS   = ['Su','Mo','Tu','We','Th','Fr','Sa'];
  const MONTH_LONG = ['January','February','March','April','May','June',
                      'July','August','September','October','November','December'];
  const MONTH_SHORT = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];

  let dpSelected = null;  // Date object
  let dpViewYear, dpViewMonth;
  let dpFocused = null;
  let dpOpen = false;
  let dpSpinnerOpen = false;

  window.initDatePicker = function() {
    const dpTrigger    = document.getElementById('dp-trigger');
    const dpPanel      = document.getElementById('dp-panel');
    const dpHidden     = document.getElementById('reportDate');
    const dpLabel      = document.getElementById('dp-label');

    if (!dpTrigger || !dpPanel || dpTrigger._dpInited) return;
    dpTrigger._dpInited = true;

    // Init: set to yesterday or hidden field value
    const yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    if (dpHidden && dpHidden.value) {
      const parts = dpHidden.value.split('-');
      if (parts.length === 3) {
        dpSelected = new Date(+parts[0], +parts[1]-1, +parts[2]);
      }
    }
    if (!dpSelected) dpSelected = yesterday;
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

    document.addEventListener('click', (e) => {
      if (dpOpen && !dpPanel.contains(e.target) && e.target !== dpTrigger) dpClose();
    });

    document.addEventListener('keydown', (e) => {
      if (dpOpen && !dpSpinnerOpen) dpHandleCalKey(e);
    });

    // Nav buttons
    document.getElementById('dp-prev')?.addEventListener('click', () => dpStepMonth(-1));
    document.getElementById('dp-next')?.addEventListener('click', () => dpStepMonth(1));
    document.getElementById('dp-today')?.addEventListener('click', () => { dpSetDate(new Date(), true); dpClose(); });
    document.getElementById('dp-clear')?.addEventListener('click', () => { dpSelected=null; dpUpdateTrigger(); dpClose(); });
    document.getElementById('dp-spin-open')?.addEventListener('click', dpOpenSpinner);
    document.getElementById('dp-spin-back')?.addEventListener('click', dpCloseSpinner);
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
    dpPanel.classList.remove('hidden');
    dpSpinnerOpen = false;
    if (dpSpinView) dpSpinView.style.display = 'none';
    if (dpGridView) dpGridView.classList.remove('hidden');
    dpViewYear  = dpSelected ? dpSelected.getFullYear()  : new Date().getFullYear();
    dpViewMonth = dpSelected ? dpSelected.getMonth()     : new Date().getMonth();
    dpFocused   = dpSelected ? new Date(dpSelected)      : null;
    dpRenderGrid();
    if (dpTrigger) dpTrigger.setAttribute('aria-expanded', 'true');
    if (window.clearFormErrors) window.clearFormErrors();
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

  function dpSetDate(d, updateView) {
    const dpHidden = document.getElementById('reportDate');
    const oldVal   = dpHidden ? dpHidden.value : '';

    dpSelected = d ? new Date(d) : null;
    dpUpdateTrigger();
    const newVal   = dpSelected ? dpIso(dpSelected) : '';
    if (dpHidden) dpHidden.value = newVal;

    if (updateView && dpSelected) {
      dpViewYear  = dpSelected.getFullYear();
      dpViewMonth = dpSelected.getMonth();
    }
    if (dpOpen) dpRenderGrid();

    const dashContainer = document.getElementById('dashboard-content');
    if (dashContainer && newVal && oldVal && oldVal !== newVal) {
      const url = '/?date=' + newVal + '&partial=true';
      if (window.htmx) {
        htmx.ajax('GET', url, { target: '#dashboard-content', swap: 'innerHTML' });
        history.pushState({ path: '/?date=' + newVal }, '', '/?date=' + newVal);
      }
    } else {
      triggerPreview();
    }

    if (window.clearFormErrors) window.clearFormErrors();
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

  function dpRenderGrid() {
    const dpGrid = document.getElementById('dp-grid');
    if (!dpGrid) return;
    // Update the visible month/year label (inside the dp-spin-open trigger button)
    const headerSpan = document.querySelector('#dp-spin-open > span');
    if (headerSpan) {
      const monthLongName = MONTH_LONG[dpViewMonth % 12] || '';
      headerSpan.textContent = `${monthLongName} ${dpViewYear}`;
    }

    const today   = new Date();
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

    dpGrid.innerHTML = cells.map(c => {
      const cd  = new Date(c.y, c.m, c.d);
      const dow = cd.getDay();
      let cls = 'cal-day';
      if (c.outside) cls += ' outside';
      else if (dow === 0 || dow === 6) cls += ' weekend';
      if (!c.outside && dpIsSame(cd, today))    cls += ' today';
      if (dpSelected && dpIsSame(cd, dpSelected)) cls += ' selected';
      if (dpFocused  && dpIsSame(cd, dpFocused))  cls += ' focused';
      return `<button type="button" class="${escapeHTML(cls)}" data-y="${Number(c.y)}" data-m="${Number(c.m)}" data-d="${Number(c.d)}">${Number(c.d)}</button>`;
    }).join('');

    dpGrid.querySelectorAll('.cal-day').forEach(btn => {
      btn.addEventListener('click', () => {
        const y = +btn.dataset.y, m = +btn.dataset.m, d = +btn.dataset.d;
        dpSetDate(new Date(y, m, d), true);
        dpClose();
      });
    });
  }

  function dpStepMonth(delta) {
    dpViewMonth += delta;
    if (dpViewMonth < 0)  { dpViewMonth = 11; dpViewYear--; }
    if (dpViewMonth > 11) { dpViewMonth = 0;  dpViewYear++; }
    dpRenderGrid();
  }

  function dpHandleCalKey(e) {
    if (!dpFocused && dpSelected) dpFocused = new Date(dpSelected);
    if (!dpFocused) dpFocused = new Date(dpViewYear, dpViewMonth, 1);

    if (e.key === 'ArrowRight') { e.preventDefault(); dpFocused.setDate(dpFocused.getDate()+1); }
    else if (e.key === 'ArrowLeft')  { e.preventDefault(); dpFocused.setDate(dpFocused.getDate()-1); }
    else if (e.key === 'ArrowDown')  { e.preventDefault(); dpFocused.setDate(dpFocused.getDate()+7); }
    else if (e.key === 'ArrowUp')    { e.preventDefault(); dpFocused.setDate(dpFocused.getDate()-7); }
    else if (e.key === 'PageDown')   { e.preventDefault(); dpStepMonth(1); return; }
    else if (e.key === 'PageUp')     { e.preventDefault(); dpStepMonth(-1); return; }
    else if (e.key === 'Enter')      { e.preventDefault(); if (dpFocused) { dpSetDate(dpFocused, true); dpClose(); } return; }
    else if (e.key === 'Escape')     { dpClose(); return; }
    else if (e.key === 't' || e.key === 'T') { dpSetDate(new Date(), true); dpClose(); return; }
    else if (e.key === 'm' || e.key === 'M') { dpOpenSpinner(); return; }
    else return;

    dpViewYear  = dpFocused.getFullYear();
    dpViewMonth = dpFocused.getMonth();
    dpRenderGrid();
  }

  // ── Spinner (month/year wheel) ──
  const ITEM_H = 32;
  const CUR_YEAR = new Date().getFullYear();
  const YEAR_LIST = [];
  for (let y = CUR_YEAR-20; y <= CUR_YEAR+5; y++) YEAR_LIST.push(y);

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
        } else if (e.key === 'ArrowDown') {
          e.preventDefault();
          const next = rowEl.nextElementSibling?.querySelector('input[name="expenseAmount[]"]');
          if (next) next.focus();
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          const prev = rowEl.previousElementSibling?.querySelector('input[name="expenseAmount[]"]');
          if (prev) prev.focus();
        }
      });
    }

    if (descInput) {
      descInput.addEventListener('keydown', e => {
        if (e.key === 'Backspace' && e.shiftKey) {
          e.preventDefault();
          rowEl.remove(); reNumberRows(); triggerPreview();
        } else if (e.key === 'ArrowDown') {
          e.preventDefault();
          const next = rowEl.nextElementSibling?.querySelector('input[name="expenseDesc[]"]');
          if (next) next.focus();
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          const prev = rowEl.previousElementSibling?.querySelector('input[name="expenseDesc[]"]');
          if (prev) prev.focus();
        }
      });
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

  // Hook the HTMX add-expense button if present; replace with local addExpenseRow
  const addBtn = document.getElementById('add-expense-btn');
  if (addBtn) {
    // Override HTMX with instant client-side row
    addBtn.removeAttribute('hx-get');
    addBtn.removeAttribute('hx-target');
    addBtn.removeAttribute('hx-swap');
    addBtn.addEventListener('click', (e) => {
      e.preventDefault();
      addExpenseRow();
    });
  }


  /* ─────────────────────────────────────────────────────
     GLOBAL KEYBOARD SHORTCUTS
  ───────────────────────────────────────────────────── */
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
    if (e.key === 'Escape') {
      if (dpOpen) { dpClose(); return; }
      closeModal();
      return;
    }

    // ? → shortcuts modal (only when not typing)
    if (e.key === '?' && !isTyping) { e.preventDefault(); toggleShortcutsModal(); return; }

    // Alt shortcuts work EVEN when a field is focused — that's the whole point
    if (e.altKey) {
      const map = { d:'dp-trigger', s:'totalSale', c:'creditSale', b:'bankTransfer', k:'counterCash', a:null };
      const k = e.key.toLowerCase();
      if (Object.prototype.hasOwnProperty.call(map, k)) {
        e.preventDefault();
        if (k === 'a') { addExpenseRow(); return; }
        const targetId = map[k];
        if (targetId) document.getElementById(targetId)?.focus();
        return;
      }
    }

    if (isTyping) return;  // non-Alt shortcuts only work outside input focus

    // Dashboard: left/right arrow to navigate dates
    const prevLink = document.getElementById('dash-prev-day');
    const nextLink = document.getElementById('dash-next-day');
    if (prevLink && nextLink) {
      if (e.key === 'ArrowLeft')  { e.preventDefault(); prevLink.click(); }
      if (e.key === 'ArrowRight') { e.preventDefault(); nextLink.click(); }
    }
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

  ['input', 'change', 'click'].forEach(evtType => {
    document.addEventListener(evtType, (e) => {
      if (e.target.closest('form') || e.target.closest('#dp-trigger') || e.target.closest('#dp-panel')) {
        window.clearFormErrors();
      }
    });
  });


  /* ─────────────────────────────────────────────────────
     THEME TOGGLE & AUTO SYSTEM OS SYNC
  ───────────────────────────────────────────────────── */
  window.getSystemTheme = window.getSystemTheme || function() {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  };

  window.getCurrentTheme = window.getCurrentTheme || function() {
    const saved = localStorage.getItem('wedrink_theme');
    if (saved === 'dark' || saved === 'light') return saved;
    return getSystemTheme();
  };

  window.applyTheme = window.applyTheme || function(theme) {
    const isDark = theme === 'dark';
    document.documentElement.classList.toggle('theme-light', !isDark);
    document.documentElement.classList.toggle('dark', isDark);
    document.documentElement.setAttribute('data-theme', isDark ? 'dark' : 'light');
    updateThemeUI(theme);
  };

  window.toggleTheme = window.toggleTheme || function() {
    const current = getCurrentTheme();
    const next = current === 'light' ? 'dark' : 'light';
    localStorage.setItem('wedrink_theme', next);
    applyTheme(next);
  };

  window.syncThemeWithSystem = window.syncThemeWithSystem || function() {
    localStorage.removeItem('wedrink_theme');
    applyTheme(getSystemTheme());
  };

  function updateThemeUI(theme) {
    const icon = document.getElementById('theme-icon');
    const label = document.getElementById('theme-label');
    if (!icon || !label) return;

    if (theme === 'light') {
      icon.textContent = '☀️';
      label.textContent = 'Light';
    } else {
      icon.textContent = '🌙';
      label.textContent = 'Dark';
    }
  }

  // Auto-sync if system OS theme changes
  const systemThemeMedia = window.matchMedia('(prefers-color-scheme: dark)');
  if (typeof systemThemeMedia.addEventListener === 'function') {
    systemThemeMedia.addEventListener('change', () => {
      syncThemeWithSystem();
    });
  } else if (typeof systemThemeMedia.addListener === 'function') {
    systemThemeMedia.addListener(() => {
      syncThemeWithSystem();
    });
  }

  applyTheme(getCurrentTheme());
  document.addEventListener('htmx:afterSwap', () => {
    applyTheme(getCurrentTheme());
  });


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

    function isInsideHorizontallyScrollable(el) {
      let curr = el;
      while (curr && curr !== document.body && curr !== document.documentElement) {
        if (curr.nodeType === 1) {
          const style = window.getComputedStyle(curr);
          const overflowX = style.overflowX;
          if ((overflowX === 'auto' || overflowX === 'scroll') && curr.scrollWidth > curr.clientWidth) {
            return true;
          }
          if (curr.classList && (
              curr.classList.contains('overflow-x-auto') ||
              curr.classList.contains('date-panel') ||
              curr.id === 'dp-panel' ||
              curr.id === 'global-shortcuts-modal' ||
              curr.id === 'report-modal-container'
          )) {
            return true;
          }
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

    document.addEventListener('touchend', (e) => {
      if (e.changedTouches.length !== 1) return;

      const duration = Date.now() - touchStartTime;
      if (duration > 600) return;

      const touch = e.changedTouches[0];
      const deltaX = touch.clientX - touchStartX;
      const deltaY = touch.clientY - touchStartY;

      if (Math.abs(deltaX) < 45 || Math.abs(deltaX) < 1.3 * Math.abs(deltaY)) {
        return;
      }

      const target = document.elementFromPoint(touch.clientX, touch.clientY) || e.target;
      if (isInsideHorizontallyScrollable(target)) {
        return;
      }

      const navLinks = Array.from(document.querySelectorAll('#mobile-nav-bar a.mobile-nav-item, nav a[href]'));
      if (navLinks.length === 0) return;

      const routes = [];
      navLinks.forEach(link => {
        const href = link.getAttribute('data-nav-href') || link.getAttribute('href');
        if (href && !routes.includes(href) && (href === '/' || href === '/submit' || href === '/reports' || href === '/admin/users' || href === '/profile')) {
          routes.push(href);
        }
      });

      if (routes.length < 2) return;

      const currentPath = window.location.pathname;
      let currentIndex = routes.indexOf(currentPath);
      if (currentIndex === -1) {
        if (currentPath === '' || currentPath === '/') currentIndex = routes.indexOf('/');
        else if (currentPath.startsWith('/submit')) currentIndex = routes.indexOf('/submit');
        else if (currentPath.startsWith('/reports')) currentIndex = routes.indexOf('/reports');
        else if (currentPath.startsWith('/admin/users')) currentIndex = routes.indexOf('/admin/users');
        else if (currentPath.startsWith('/profile')) currentIndex = routes.indexOf('/profile');
      }

      if (currentIndex === -1) return;

      let targetIndex = -1;
      let dir = 'left';
      if (deltaX < 0 && currentIndex < routes.length - 1) {
        targetIndex = currentIndex + 1;
        dir = 'left';
      } else if (deltaX > 0 && currentIndex > 0) {
        targetIndex = currentIndex - 1;
        dir = 'right';
      }

      if (targetIndex !== -1 && targetIndex !== currentIndex) {
        window.navigateToTabInstant(routes[targetIndex], dir);
      }
    }, { passive: true });
  })();

});
