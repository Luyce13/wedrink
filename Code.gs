/**
 * Serves the HTML frontend interface.
 */
function doGet(e) {
  return HtmlService.createHtmlOutputFromFile('Index')
      .setTitle('End of Day Report')
      .addMetaTag('viewport', 'width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no')
      .setXFrameOptionsMode(HtmlService.XFrameOptionsMode.ALLOWALL);
}

/**
 * Adds custom menu options to the Google Sheet UI upon opening.
 * Automatically refreshes the dashboard and exposes legacy migration tool.
 */
function onOpen() {
  SpreadsheetApp.getUi()
    .createMenu('EOD Reports')
    .addItem('Refresh Dashboard', 'loadDashboard')
    .addItem('Initialize / Restore Current Month', 'initializeAllSheets')
    .addSeparator()
    .addItem('Remove All ".00" Decimals Across Sheets', 'fixAllSheetsFormatting')
    .addItem('Migrate Legacy Data to Monthly Sheets', 'migrateLegacyDataToMonthlySheets')
    .addToUi();

  ensureDashboardSheet_();
  fixAllSheetsFormatting(); // Cleans existing .00 decimals automatically across all monthly sheets
  loadDashboard(); // Auto-loads data instantly on file open
}

/**
 * Initializes the current month's sheets and sets up/updates the Dashboard.
 */
function initializeAllSheets() {
  var now = new Date();
  var summaryName = getMonthlySheetName_('EOD Summary', now);
  var expenseName = getMonthlySheetName_('EOD Expenses', now);

  getOrCreateSheet_(summaryName,
    ['Date', 'Total Sale', 'Credit Card Sale', 'Bank Transfer', 'Other Payments', 'Expected Cash', 'Counter Cash', 'Difference', 'Report ID']);
  getOrCreateSheet_(expenseName,
    ['Report ID', 'Date', 'Description', 'Amount']);

  ensureDashboardSheet_();
  loadDashboard();
}

/**
 * Formats a base name and date into a monthly sheet name (e.g. "EOD Summary - 2026-07").
 * @param {string} baseName The prefix of the sheet (e.g., "EOD Summary").
 * @param {Date} date The target date.
 * @return {string} The fully formatted sheet name.
 */
function getMonthlySheetName_(baseName, date) {
  var year = date.getFullYear();
  var month = ('0' + (date.getMonth() + 1)).slice(-2); // Pads month to two digits (e.g. "07")
  return baseName + ' - ' + year + '-' + month;
}

/**
 * Helper to get an existing sheet or create it with formatted headers if missing.
 */
function getOrCreateSheet_(name, headers) {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var sheet = ss.getSheetByName(name);
  if (!sheet) {
    sheet = ss.insertSheet(name);
    sheet.appendRow(headers);
    sheet.getRange(1, 1, 1, headers.length)
      .setFontWeight('bold')
      .setFontSize(12)
      .setBackground('#1e293b') // Elegant Dark Slate
      .setFontColor('#ffffff');
    sheet.setFrozenRows(1);
    sheet.autoResizeColumns(1, headers.length);
  }
  return sheet;
}

/**
 * Utility to compare if two date objects fall on the exact same calendar day.
 */
function sameDay_(d1, d2) {
  return d1.getFullYear() === d2.getFullYear() &&
         d1.getMonth() === d2.getMonth() &&
         d1.getDate() === d2.getDate();
}

/**
 * Applies alternating zebra striping (white / light tint), horizontal borders, and #,##0 formatting to data rows.
 */
function applyZebraStriping_(sheet) {
  var lastRow = sheet.getLastRow();
  var lastCol = sheet.getLastColumn();
  if (lastRow <= 1 || lastCol < 1) return;

  var range = sheet.getRange(2, 1, lastRow - 1, lastCol);
  var backgrounds = [];
  for (var r = 2; r <= lastRow; r++) {
    var rowBg = (r % 2 === 0) ? '#ffffff' : '#f8fafc'; // Subtle alternating zebra tint
    var rowColors = [];
    for (var c = 1; c <= lastCol; c++) {
      rowColors.push(rowBg);
    }
    backgrounds.push(rowColors);
  }
  range.setBackgrounds(backgrounds);
  range.setBorder(true, true, true, true, false, true, '#cbd5e1', SpreadsheetApp.BorderStyle.SOLID);

  var name = sheet.getName();
  if (name.indexOf('EOD Summary') !== -1) {
    sheet.getRange(2, 2, lastRow - 1, 7).setNumberFormat('#,##0');
  } else if (name.indexOf('EOD Expenses') !== -1) {
    sheet.getRange(2, 4, lastRow - 1, 1).setNumberFormat('#,##0');
  } else if (lastCol >= 2) {
    sheet.getRange(2, 2, lastRow - 1, lastCol - 1).setNumberFormat('#,##0');
  }
}

/**
 * Utility function to clean and fix number formatting across all monthly EOD sheets.
 * Re-formats all existing numerical cells to '#,##0' (no decimals).
 */
function fixAllSheetsFormatting() {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var sheets = ss.getSheets();
  sheets.forEach(function(s) {
    var name = s.getName();
    if (name.indexOf('EOD Summary') !== -1 || name.indexOf('EOD Expenses') !== -1) {
      applyZebraStriping_(s);
    }
  });
}

/**
 * Processes incoming EOD submissions, routing them dynamically to their respective monthly sheets.
 * Prevents duplicates for a given date within the monthly target sheet.
 */
function processForm(formData) {
  var lock = LockService.getScriptLock();
  lock.waitLock(15000); // Wait up to 15 seconds for a lock to clear concurrency issues

  try {
    if (!formData.reportDate) {
      throw new Error('Please select a report date.');
    }

    var reportDate = new Date(formData.reportDate + 'T00:00:00');
    if (isNaN(reportDate)) {
      throw new Error('Invalid report date.');
    }

    // Determine target monthly sheet names based on submitted report date
    var summarySheetName = getMonthlySheetName_('EOD Summary', reportDate);
    var expenseSheetName = getMonthlySheetName_('EOD Expenses', reportDate);

    var summarySheet = getOrCreateSheet_(summarySheetName,
      ['Date', 'Total Sale', 'Credit Card Sale', 'Bank Transfer', 'Other Payments', 'Expected Cash', 'Counter Cash', 'Difference', 'Report ID']);
    var expenseSheet = getOrCreateSheet_(expenseSheetName,
      ['Report ID', 'Date', 'Description', 'Amount']);

    // Block duplicate submissions inside the targeted monthly summary sheet
    if (summarySheet.getLastRow() > 1) {
      var existingDates = summarySheet.getRange(2, 1, summarySheet.getLastRow() - 1, 1).getValues();
      for (var r = 0; r < existingDates.length; r++) {
        var ts = existingDates[r][0];
        if (ts instanceof Date && sameDay_(ts, reportDate)) {
          throw new Error('Only one report per date is allowed.\n Already submitted for ' +
            Utilities.formatDate(reportDate, Session.getScriptTimeZone(), 'dd/MM/yyyy') + ' inside "' + summarySheetName + '".');
        }
      }
    }

    var totalSale = Math.round(parseFloat(formData.totalSale));
    var creditSale = Math.round(parseFloat(formData.creditSale));
    var bankTransfer = Math.round(parseFloat(formData.bankTransfer));
    var counterCash = Math.round(parseFloat(formData.counterCash || 0));

    if ([totalSale, creditSale, bankTransfer, counterCash].some(isNaN)) {
      throw new Error('Please enter valid numerical amounts.');
    }

    var descriptions = formData.expenseDesc;
    var amounts = formData.expenseAmount;
    if (!Array.isArray(descriptions)) {
      descriptions = descriptions ? [descriptions] : [];
      amounts = amounts ? [amounts] : [];
    }

    var reportId = Utilities.getUuid();
    var totalExpenses = 0;

    // Append dynamic expenses to the month's specific expense sheet (stored as positive whole integers)
    for (var i = 0; i < descriptions.length; i++) {
      var desc = (descriptions[i] || '').trim();
      var amt = parseFloat(amounts[i]);
      if (desc && !isNaN(amt)) {
        var posAmt = Math.round(Math.abs(amt));
        totalExpenses += posAmt;
        expenseSheet.appendRow([reportId, reportDate, desc, posAmt]);
      }
    }

    var netDailyCash = Math.round(totalSale - totalExpenses - creditSale - bankTransfer);
    var difference = Math.round(counterCash - netDailyCash);

    // Append standard EOD summary record to the monthly summary sheet
    summarySheet.appendRow([reportDate, totalSale, creditSale, bankTransfer, totalExpenses, netDailyCash, counterCash, difference, reportId]);

    var lastRow = summarySheet.getLastRow();
    summarySheet.getRange(lastRow, 1).setNumberFormat('dd/mm/yyyy');
    summarySheet.getRange(lastRow, 2, 1, 7).setNumberFormat('#,##0');

    if (netDailyCash < 0) {
      summarySheet.getRange(lastRow, 6).setFontColor('#ef4444').setFontWeight('bold');
    }
    if (difference !== 0) {
      summarySheet.getRange(lastRow, 8).setFontColor('#ef4444').setFontWeight('bold');
    }

    // Apply alternating zebra striping to data rows with values
    applyZebraStriping_(summarySheet);
    applyZebraStriping_(expenseSheet);

    // Instantly refresh dashboard if the current view date matches the submitted date
    var ss = SpreadsheetApp.getActiveSpreadsheet();
    var dash = ss.getSheetByName('Dashboard');
    if (dash) {
      var currentDashDate = dash.getRange('B3').getValue();
      if (currentDashDate instanceof Date && sameDay_(currentDashDate, reportDate)) {
        loadDashboard();
      }
    }

    return 'Report submitted successfully to "' + summarySheetName + '"!';
  } finally {
    lock.releaseLock();
  }
}

/**
 * Builds or rebuilds the Dashboard layout structure with optimal visual standards.
 */
function ensureDashboardSheet_() {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var sheet = ss.getSheetByName('Dashboard');
  var isNew = !sheet;
  if (isNew) sheet = ss.insertSheet('Dashboard', 0);

  var existingDate = isNew ? null : sheet.getRange('B3').getValue();
  var dateToSet = (existingDate instanceof Date && !isNaN(existingDate)) ? existingDate : new Date();
  dateToSet.setHours(0, 0, 0, 0);

  // Clear sheet protections to rewrite cells safely
  sheet.getProtections(SpreadsheetApp.ProtectionType.RANGE).forEach(function(p) { p.remove(); });
  sheet.getProtections(SpreadsheetApp.ProtectionType.SHEET).forEach(function(p) { p.remove(); });

  sheet.clear();
  sheet.setHiddenGridlines(true); // Programmatically hides default gridlines completely

  // Adjust Rows Heights for clear visual hierarchies
  sheet.setRowHeight(1, 55);  // Modern Header
  sheet.setRowHeight(2, 15);  // Spacer
  sheet.setRowHeight(3, 28);  // Date picker
  sheet.setRowHeight(4, 15);  // Spacer
  sheet.setRowHeight(5, 26);  // Section Subheader
  sheet.setRowHeight(13, 15); // Spacer
  sheet.setRowHeight(14, 26); // Section Subheader

  // Setup Modern Header (Row 1)
  sheet.getRange('A1:B1').merge()
    .setValue('DAYEND REPORT')
    .setBackground('#0f172a') // Deep Midnight Blue
    .setFontColor('#f8fafc')   // Off-White
    .setFontSize(14)
    .setFontWeight('bold')
    .setHorizontalAlignment('center')
    .setVerticalAlignment('middle');

  // Date Selection Row (Row 3)
  sheet.getRange('A3').setValue('Select Report Date:').setFontWeight('bold').setFontSize(12).setFontColor('#475569').setVerticalAlignment('middle');
  sheet.getRange('B3').setValue(dateToSet).setNumberFormat('dd/mm/yyyy')
    .setBackground('#fef08a') // Subtle gold highlight
    .setBorder(true, true, true, true, false, false, '#e2e8f0', SpreadsheetApp.BorderStyle.SOLID)
    .setFontWeight('bold')
    .setFontSize(12)
    .setHorizontalAlignment('center')
    .setVerticalAlignment('middle');

  sheet.getRange('B3').setDataValidation(
    SpreadsheetApp.newDataValidation().requireDate().setAllowInvalid(false).build()
  );

  // Section: Summary Statistics (Rows 5-10)
  sheet.getRange('A5:B5').merge().setValue('SUMMARY STATISTICS')
    .setBackground('#e2e8f0').setFontColor('#0f172a').setFontWeight('bold').setFontSize(12).setHorizontalAlignment('center').setVerticalAlignment('middle');

  var labels = [
    ['Total Sales', 'A6'],
    ['Credit Card Sales', 'A7'],
    ['Bank Transfers', 'A8'],
    ['Other Payments', 'A9'],
    ['Expected Closing Cash', 'A10']
  ];
  labels.forEach(function(item) {
    sheet.getRange(item[1]).setValue(item[0]).setFontWeight('bold').setFontSize(12).setFontColor('#334155');
  });

  // Physical Reconciliation (Rows 11-12)
  sheet.getRange('A11').setValue('Actual TILL Cash (Physical)').setFontWeight('bold').setFontSize(12).setFontColor('#334155');
  sheet.getRange('A12').setValue('Cash Discrepancy (Difference)').setFontWeight('bold').setFontSize(12).setFontColor('#334155');

  // Explicitly set font size 12 across all Summary & Reconciliation cells
  sheet.getRange('A6:B12').setFontSize(12);

  // Apply alternating Zebra backgrounds on Rows 6 to 12
  var summaryBgs = [
    ['#ffffff', '#ffffff'], // Row 6
    ['#f8fafc', '#f8fafc'], // Row 7
    ['#ffffff', '#ffffff'], // Row 8
    ['#f8fafc', '#f8fafc'], // Row 9
    ['#f1f5f9', '#f1f5f9'], // Row 10 (Expected Closing Cash highlight)
    ['#ffffff', '#ffffff'], // Row 11
    ['#f8fafc', '#f8fafc']  // Row 12
  ];
  sheet.getRange('A6:B12').setBackgrounds(summaryBgs);

  // Apply horizontal borders across Rows 5 to 12
  sheet.getRange('A5:B12').setBorder(true, true, true, true, false, true, '#cbd5e1', SpreadsheetApp.BorderStyle.SOLID);

  // Section: Other Expenses Breakdown (Rows 14-15)
  sheet.getRange('A14:B14').merge().setValue('OTHER PAYMENTS BREAKDOWN')
    .setBackground('#e2e8f0').setFontColor('#0f172a').setFontWeight('bold').setFontSize(12).setHorizontalAlignment('center').setVerticalAlignment('middle');

  sheet.getRange('A15:B15').setValues([['Description', 'Amount']]).setFontWeight('bold').setFontSize(12)
    .setBackground('#3b82f6').setFontColor('#ffffff').setHorizontalAlignment('center');

  sheet.getRange('A14:B15').setBorder(true, true, true, true, false, true, '#cbd5e1', SpreadsheetApp.BorderStyle.SOLID);

  sheet.setColumnWidth(1, 250);
  sheet.setColumnWidth(2, 170);

  // Clean Unused Columns & Rows (Hides noise completely)
  var maxRows = sheet.getMaxRows();
  var maxCols = sheet.getMaxColumns();
  if (maxRows > 45) {
    sheet.deleteRows(46, maxRows - 45);
  }
  if (maxCols > 2) {
    sheet.deleteColumns(3, maxCols - 2);
  }

  // Entirely lock sheet, except cell B3
  var protection = sheet.protect().setDescription('Dashboard completely locked except Date Picker');
  protection.setUnprotectedRanges([sheet.getRange('B3')]);

  var me = Session.getEffectiveUser();
  if (me) {
    protection.addEditor(me);
    var editors = protection.getEditors();
    for (var i = 0; i < editors.length; i++) {
      if (editors[i].getEmail() !== me.getEmail()) {
        protection.removeEditor(editors[i]);
      }
    }
  }

  return sheet;
}

/**
 * Automatically triggers loadDashboard whenever the user changes the Date input on cell B3.
 */
function onEdit(e) {
  if (!e || !e.range) return;
  var sheet = e.range.getSheet();
  if (sheet.getName() === 'Dashboard' && e.range.getA1Notation() === 'B3') {
    loadDashboard();
  }
}

/**
 * Loads and displays metrics on the Dashboard by identifying the correct monthly source sheets.
 */
function loadDashboard() {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var dash = ss.getSheetByName('Dashboard') || ensureDashboardSheet_();

  var targetDate = dash.getRange('B3').getValue();
  if (!(targetDate instanceof Date) || isNaN(targetDate)) {
    dash.getRange('B4').setValue('Please pick a valid date');
    return;
  }
  targetDate.setHours(0, 0, 0, 0);

  // Dynamically resolve target sheet names for the chosen month
  var summarySheetName = getMonthlySheetName_('EOD Summary', targetDate);
  var expenseSheetName = getMonthlySheetName_('EOD Expenses', targetDate);

  var summarySheet = ss.getSheetByName(summarySheetName);
  var expenseSheet = ss.getSheetByName(expenseSheetName);

  // Reset metrics cell states
  dash.getRange('B6:B10').clearContent();
  dash.getRange('B11:B12').clearContent();

  // Clear any existing breakdowns in row 16 or lower
  var lastRow = dash.getLastRow();
  if (lastRow >= 16) {
    dash.getRange(16, 1, lastRow - 15, 2).clearContent().setBackground(null).setFontColor(null).setFontWeight(null);
  }

  // Handle cases where the sheet for the selected month does not exist yet
  if (!summarySheet || summarySheet.getLastRow() < 2) {
    dash.getRange('B6').setValue('No records for this month').setFontColor('#64748b');
    dash.getRange('A16').setValue('No records submitted for this date.').setFontColor('#94a3b8').setFontStyle('italic');
    return;
  }

  var summaryData = summarySheet.getRange(2, 1, summarySheet.getLastRow() - 1, 9).getValues();
  var totals = { sale: 0, credit: 0, bank: 0, expenses: 0, counterCash: 0 };
  var matchedReportIds = [];
  var dayFound = false;

  // Filter and extract summaries that match the exact selected date
  summaryData.forEach(function(row) {
    var d = row[0];
    if (d instanceof Date && sameDay_(d, targetDate)) {
      totals.sale += row[1];
      totals.credit += row[2];
      totals.bank += row[3];
      totals.expenses += row[4];
      totals.counterCash += (row[6] || 0);
      matchedReportIds.push(row[8]);
      dayFound = true;
    }
  });

  var closingCash = totals.sale - totals.credit - totals.bank + totals.expenses;
  var difference = totals.counterCash - closingCash;

  // Render statistical calculations on the Dashboard
  dash.getRange('A6:B12').setFontSize(12);
  dash.getRange('B6').setValue(totals.sale).setNumberFormat('#,##0').setFontColor('#1e293b');
  dash.getRange('B7').setValue(totals.credit).setNumberFormat('#,##0').setFontColor('#475569');
  dash.getRange('B8').setValue(totals.bank).setNumberFormat('#,##0').setFontColor('#475569');
  dash.getRange('B9').setValue(Math.abs(totals.expenses)).setNumberFormat('#,##0').setFontColor('#475569');
  dash.getRange('B10').setValue(closingCash).setNumberFormat('#,##0')
    .setFontWeight('bold')
    .setFontColor(closingCash < 0 ? '#ef4444' : '#15803d');

  // Render physical reconciliation metrics (Rows 11 & 12)
  dash.getRange('B11').setValue(dayFound ? totals.counterCash : 0).setNumberFormat('#,##0').setFontColor('#1e293b');

  var diffCell = dash.getRange('B12');
  diffCell.setValue(dayFound ? difference : 0).setNumberFormat('#,##0').setFontWeight('bold');
  if (!dayFound || difference === 0) {
    diffCell.setFontColor('#15803d'); // Balanced Match
  } else if (difference < 0) {
    diffCell.setFontColor('#ef4444'); // Shortage (Red)
  } else {
    diffCell.setFontColor('#2563eb'); // Surplus (Blue)
  }

  // If no submissions exist for the selected day, inform user and exit
  if (matchedReportIds.length === 0) {
    dash.getRange('A16').setValue('No records submitted for this date.').setFontColor('#94a3b8').setFontStyle('italic');
    return;
  }

  // Look up dynamic other payment expenses from the month's respective expense sheet
  if (expenseSheet && expenseSheet.getLastRow() > 1) {
    var expData = expenseSheet.getRange(2, 1, expenseSheet.getLastRow() - 1, 4).getValues();
    var rowsOut = [];
    expData.forEach(function(row) {
      if (matchedReportIds.indexOf(row[0]) !== -1) {
        rowsOut.push([row[2], Math.abs(row[3])]); // Positive expense amount
      }
    });
    if (rowsOut.length) {
      var writeRange = dash.getRange(16, 1, rowsOut.length, 2);
      writeRange.setValues(rowsOut);
      dash.getRange(16, 2, rowsOut.length, 1).setNumberFormat('#,##0');
      writeRange.setFontColor('#334155').setFontSize(11);

      // Apply zebra pattern and horizontal borders to breakdown rows!
      var bgs = [];
      for (var r = 0; r < rowsOut.length; r++) {
        var rowBg = (r % 2 === 0) ? '#ffffff' : '#f8fafc';
        bgs.push([rowBg, rowBg]);
      }
      writeRange.setBackgrounds(bgs);
      writeRange.setBorder(true, true, true, true, false, true, '#cbd5e1', SpreadsheetApp.BorderStyle.SOLID);
    } else {
      dash.getRange('A16').setValue('No other payments recorded.').setFontColor('#94a3b8').setFontStyle('italic');
    }
  } else {
    dash.getRange('A16').setValue('No other payments recorded.').setFontColor('#94a3b8').setFontStyle('italic');
  }
}

/**
 * One-time utility to migrate data from legacy single sheets ('EOD Summary' and 'EOD Expenses')
 * into their respective monthly rotated sheets based on row dates.
 * Legacy sheets are kept and renamed with a TIMESTAMP tag to ensure zero data loss.
 */
function migrateLegacyDataToMonthlySheets() {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var legacySummary = ss.getSheetByName('EOD Summary');
  var legacyExpenses = ss.getSheetByName('EOD Expenses');

  var migratedSummariesCount = 0;
  var migratedExpensesCount = 0;

  if (!legacySummary && !legacyExpenses) {
    SpreadsheetApp.getUi().alert('No legacy "EOD Summary" or "EOD Expenses" sheets were found to migrate.');
    return;
  }

  // 1. Migrate EOD Summary data rows dynamically to their monthly sheets
  if (legacySummary && legacySummary.getLastRow() > 1) {
    var summaryData = legacySummary.getRange(2, 1, legacySummary.getLastRow() - 1, 9).getValues();
    summaryData.forEach(function(row) {
      var dateVal = row[0];
      if (dateVal instanceof Date && !isNaN(dateVal)) {
        var targetSheetName = getMonthlySheetName_('EOD Summary', dateVal);
        var targetSheet = getOrCreateSheet_(targetSheetName,
          ['Date', 'Total Sale', 'Credit Card Sale', 'Bank Transfer', 'Other Payments', 'Expected Cash', 'Counter Cash', 'Difference', 'Report ID']);

        // Skip copy if entry already exists in target monthly sheet to prevent accidental duplicates
        var exists = false;
        if (targetSheet.getLastRow() > 1) {
          var targetDates = targetSheet.getRange(2, 1, targetSheet.getLastRow() - 1, 1).getValues();
          for (var i = 0; i < targetDates.length; i++) {
            if (targetDates[i][0] instanceof Date && sameDay_(targetDates[i][0], dateVal)) {
              exists = true;
              break;
            }
          }
        }

        if (!exists) {
          targetSheet.appendRow(row);
          var lastRow = targetSheet.getLastRow();
          targetSheet.getRange(lastRow, 1).setNumberFormat('dd/mm/yyyy');
          targetSheet.getRange(lastRow, 2, 1, 8).setNumberFormat('#,##0');
          if (row[5] < 0) targetSheet.getRange(lastRow, 6).setFontColor('#ef4444').setFontWeight('bold');
          if (row[7] !== 0) targetSheet.getRange(lastRow, 8).setFontColor('#ef4444').setFontWeight('bold');
          migratedSummariesCount++;
        }
      }
    });
  }

  // 2. Migrate EOD Expenses data rows dynamically to their monthly sheets
  if (legacyExpenses && legacyExpenses.getLastRow() > 1) {
    var expenseData = legacyExpenses.getRange(2, 1, legacyExpenses.getLastRow() - 1, 4).getValues();
    expenseData.forEach(function(row) {
      var dateVal = row[1]; // Index 1 is the Date column
      if (dateVal instanceof Date && !isNaN(dateVal)) {
        var targetSheetName = getMonthlySheetName_('EOD Expenses', dateVal);
        var targetSheet = getOrCreateSheet_(targetSheetName,
          ['Report ID', 'Date', 'Description', 'Amount']);

        // Skip duplicate records in target monthly sheet (by Report ID + Description + Amount check)
        var exists = false;
        if (targetSheet.getLastRow() > 1) {
          var targetData = targetSheet.getRange(2, 1, targetSheet.getLastRow() - 1, 4).getValues();
          for (var i = 0; i < targetData.length; i++) {
            if (targetData[i][0] === row[0] && targetData[i][2] === row[2] && targetData[i][3] === row[3]) {
              exists = true;
              break;
            }
          }
        }

        if (!exists) {
          targetSheet.appendRow(row);
          applyZebraStriping_(targetSheet);
          migratedExpensesCount++;
        }
      }
    });
  }

  // 3. Rename legacy sheets to archive backups and prevent future route interference
  var timestamp = Utilities.formatDate(new Date(), Session.getScriptTimeZone(), 'yyyyMMdd_HHmmss');
  if (legacySummary) {
    legacySummary.setName('LEGACY_EOD_Summary_Backup_' + timestamp);
  }
  if (legacyExpenses) {
    legacyExpenses.setName('LEGACY_EOD_Expenses_Backup_' + timestamp);
  }

  SpreadsheetApp.getUi().alert(
    'Migration complete!\n\n' +
    '• Migrated Summaries: ' + migratedSummariesCount + ' records.\n' +
    '• Migrated Expenses: ' + migratedExpensesCount + ' records.\n\n' +
    'Your original sheets have been preserved and archived safely as backups.'
  );

  loadDashboard();
}