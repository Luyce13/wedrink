document.addEventListener('DOMContentLoaded', () => {
  // HTMX BeforeSwap error handling
  document.body.addEventListener('htmx:beforeSwap', function(evt) {
    if (evt.detail.xhr.status === 400 || evt.detail.xhr.status === 422) {
      evt.detail.shouldSwap = true;
      evt.detail.isError = false;
    }
  });

  // HTMX AfterSwap scroll handler
  document.body.addEventListener('htmx:afterSwap', function(evt) {
    const alertDiv = document.getElementById('form-alert');
    if (alertDiv && alertDiv.innerText.trim() !== '') {
      window.scrollTo({ top: 0, behavior: 'smooth' });
    }
  });

  // Global Keyboard Shortcuts (Ctrl+Enter to submit, ? for help)
  document.addEventListener('keydown', (e) => {
    const isTyping = ['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName);

    // Ctrl+Enter or Cmd+Enter to submit form
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      const form = document.querySelector('form[hx-post="/reports"]');
      if (form) {
        e.preventDefault();
        htmx.trigger(form, 'submit');
      }
    }

    // Escape closes modals
    if (e.key === 'Escape') {
      window.closeModal();
    }

    // ? toggles shortcuts help
    if (e.key === '?' && !isTyping) {
      e.preventDefault();
      window.toggleShortcutsModal();
    }
  });

  // Modal Close Helper
  window.closeModal = function() {
    const modal = document.getElementById('report-modal-container');
    if (modal) {
      modal.innerHTML = '';
    }
    const shortcutsModal = document.getElementById('global-shortcuts-modal');
    if (shortcutsModal) {
      shortcutsModal.remove();
    }
  };

  // Toggle Keyboard Shortcuts Overlay
  window.toggleShortcutsModal = function() {
    let modal = document.getElementById('global-shortcuts-modal');
    if (modal) {
      modal.remove();
      return;
    }

    modal = document.createElement('div');
    modal.id = 'global-shortcuts-modal';
    modal.className = 'fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm';
    modal.innerHTML = `
      <div class="glass-card w-full max-w-md p-6 bg-slate-950 border border-slate-800 rounded-lg shadow-2xl space-y-4">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <h2 class="text-sm font-bold text-white flex items-center gap-2">
            <span>Keyboard Shortcuts</span>
          </h2>
          <button onclick="closeModal()" class="text-slate-400 hover:text-white text-xs">✕</button>
        </div>

        <div class="space-y-2 text-xs divide-y divide-slate-800/60">
          <div class="flex justify-between py-1.5"><span class="text-slate-400">Submit Form</span><kbd class="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded text-indigo-300 font-mono">Ctrl + Enter</kbd></div>
          <div class="flex justify-between py-1.5"><span class="text-slate-400">Add New Expense Row</span><kbd class="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded text-amber-300 font-mono">Enter (in Amount field)</kbd></div>
          <div class="flex justify-between py-1.5"><span class="text-slate-400">Navigate Expense Rows</span><kbd class="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded text-slate-300 font-mono">↑ / ↓ Arrow Keys</kbd></div>
          <div class="flex justify-between py-1.5"><span class="text-slate-400">Delete Current Expense Row</span><kbd class="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded text-red-300 font-mono">Shift + Backspace</kbd></div>
          <div class="flex justify-between py-1.5"><span class="text-slate-400">Close Modal / Overlay</span><kbd class="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded text-slate-300 font-mono">Escape</kbd></div>
          <div class="flex justify-between py-1.5"><span class="text-slate-400">Toggle Shortcuts Guide</span><kbd class="px-2 py-0.5 bg-slate-800 border border-slate-700 rounded text-cyan-300 font-mono">?</kbd></div>
        </div>

        <div class="pt-2 text-center">
          <button onclick="closeModal()" class="btn btn-primary text-xs py-1.5 px-4">Close Guide</button>
        </div>
      </div>
    `;
    document.body.appendChild(modal);
  };

  // Universal Bulletproof Copy Helper (Works in HTTPS, HTTP, Localhost, & all Browsers)
  window.copyTextToClipboard = function(text, successMsg) {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(() => {
        showToast(successMsg);
      }).catch(() => {
        fallbackCopyText(text, successMsg);
      });
    } else {
      fallbackCopyText(text, successMsg);
    }
  };

  function fallbackCopyText(text, successMsg) {
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.top = '-9999px';
    textArea.style.left = '-9999px';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
      const successful = document.execCommand('copy');
      if (successful) {
        showToast(successMsg);
      } else {
        window.prompt('Copy payload text:', text);
      }
    } catch (err) {
      window.prompt('Copy payload text:', text);
    }
    document.body.removeChild(textArea);
  }

  // Universal Single Copy Button for Dashboard Summary
  window.copyUniversalSummary = function() {
    const data = getDashboardData();
    if (!data) return;

    let expSection = '  • None recorded';
    if (data.expenses.length > 0) {
      expSection = data.expenses.map(e => `  • ${e.desc}: Rs. ${e.amt}`).join('\n');
    }

    const payload = 
`📊 WEDRINK END OF DAY SUMMARY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📅 Date: ${data.date}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
💰 Total Sales:         Rs. ${data.totalSale}
💳 Credit Card Sales:    Rs. ${data.creditSale}
🏦 Bank Transfers:      Rs. ${data.bankTransfer}
📦 Other Payments:       Rs. ${data.otherPayments}
💵 Expected Cash:        Rs. ${data.expectedCash}
🪙 Actual Till Cash:     Rs. ${data.counterCash}
⚡ Cash Discrepancy:     Rs. ${data.discrepancy}

📋 OTHER PAYMENTS BREAKDOWN:
${expSection}

👤 Submitted By: ${data.submittedBy}
📝 Remarks: ${data.notes}
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`;

    copyTextToClipboard(payload, '✓ Summary copied to clipboard!');
  };

  // Simple "Copy" for Expenses Breakdown (Populates 2 Columns in Excel / QuickBooks)
  window.copyExpensesForExcel = function() {
    const data = getDashboardData();
    if (!data || !data.expenses || data.expenses.length === 0) {
      alert('No itemized expenses recorded to copy.');
      return;
    }

    let tsv = `Description\tAmount\n`;
    data.expenses.forEach(exp => {
      tsv += `${exp.desc}\t${exp.amt}\n`;
    });

    copyTextToClipboard(tsv, '✓ Expenses copied! Paste directly into Excel.');
  };

  function getDashboardData() {
    const dateEl = document.getElementById('dash-date-val');
    if (!dateEl) {
      alert('No active report available on dashboard.');
      return null;
    }

    const getVal = (id) => {
      const el = document.getElementById(id);
      if (!el) return '0';
      return el.getAttribute('data-raw') || el.innerText.replace(/[^0-9.-]/g, '');
    };

    const date = dateEl.innerText.trim();
    const totalSale = getVal('dash-total-sale');
    const creditSale = getVal('dash-credit-sale');
    const bankTransfer = getVal('dash-bank-transfer');
    const otherPayments = getVal('dash-other-payments');
    const expectedCash = getVal('dash-expected-cash');
    const counterCash = getVal('dash-counter-cash');
    const discrepancy = document.getElementById('dash-discrepancy')?.innerText.trim() || '0';
    const submittedBy = document.getElementById('dash-submitted-by')?.innerText.trim() || 'N/A';
    const notes = document.getElementById('dash-notes')?.innerText.trim() || 'None';

    const expenses = [];
    document.querySelectorAll('.dash-expense-item').forEach(item => {
      const desc = item.getAttribute('data-desc') || item.querySelector('.dash-exp-desc')?.innerText.trim();
      const amt = item.getAttribute('data-amount') || item.querySelector('.dash-exp-amt')?.innerText.replace(/[^0-9.-]/g, '');
      if (desc && amt) {
        expenses.push({ desc, amt });
      }
    });

    return {
      date,
      totalSale,
      creditSale,
      bankTransfer,
      otherPayments,
      expectedCash,
      counterCash,
      discrepancy,
      submittedBy,
      notes,
      expenses
    };
  }

  // Floating Top Toast Notification
  window.showToast = function(msg) {
    const existing = document.getElementById('floating-toast-notice');
    if (existing) existing.remove();

    const toast = document.createElement('div');
    toast.id = 'floating-toast-notice';
    toast.className = 'fixed top-4 left-1/2 -translate-x-1/2 z-[9999] bg-[#131f3a] border border-[#00b4d8] text-cyan-300 px-5 py-3 rounded-lg shadow-2xl flex items-center gap-3 text-xs font-bold animate-bounce-short';
    toast.innerHTML = `
      <span>${msg}</span>
      <button onclick="this.parentElement.remove()" class="text-slate-400 hover:text-white ml-2 text-sm">✕</button>
    `;
    document.body.appendChild(toast);
    setTimeout(() => {
      if (toast.parentElement) toast.remove();
    }, 4000);
  };

  // Keyboard navigation setup for expense input rows
  window.bindExpenseRowNavigation = function(rowEl) {
    if (!rowEl) return;
    const descInput = rowEl.querySelector('input[name="expenseDesc[]"]');
    const amtInput = rowEl.querySelector('input[name="expenseAmount[]"]');

    if (!amtInput) return;

    amtInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey) {
        e.preventDefault();
        const addBtn = document.getElementById('add-expense-btn');
        if (addBtn) {
          addBtn.click();
          setTimeout(() => {
            const container = document.getElementById('expenses-container');
            if (container) {
              const lastRow = container.lastElementChild;
              const lastDesc = lastRow?.querySelector('input[name="expenseDesc[]"]');
              if (lastDesc) lastDesc.focus();
            }
          }, 50);
        }
      } else if (e.key === 'ArrowDown') {
        const nextRow = rowEl.nextElementSibling;
        if (nextRow) {
          e.preventDefault();
          const target = nextRow.querySelector('input[name="expenseAmount[]"]');
          if (target) target.focus();
        }
      } else if (e.key === 'ArrowUp') {
        const prevRow = rowEl.previousElementSibling;
        if (prevRow) {
          e.preventDefault();
          const target = prevRow.querySelector('input[name="expenseAmount[]"]');
          if (target) target.focus();
        }
      }
    });

    if (descInput) {
      descInput.addEventListener('keydown', (e) => {
        if (e.key === 'Backspace' && e.shiftKey) {
          e.preventDefault();
          rowEl.remove();
        } else if (e.key === 'ArrowDown') {
          const nextRow = rowEl.nextElementSibling;
          if (nextRow) {
            e.preventDefault();
            const target = nextRow.querySelector('input[name="expenseDesc[]"]');
            if (target) target.focus();
          }
        } else if (e.key === 'ArrowUp') {
          const prevRow = rowEl.previousElementSibling;
          if (prevRow) {
            e.preventDefault();
            const target = prevRow.querySelector('input[name="expenseDesc[]"]');
            if (target) target.focus();
          }
        }
      });
    }
  };

  document.querySelectorAll('#expenses-container > div').forEach(row => {
    bindExpenseRowNavigation(row);
  });
});
