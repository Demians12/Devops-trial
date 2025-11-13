function openBookingModal({ version, date, start, professional, specialty, unit, room }) {
  const scheduleDate = formatDateLabel(date);
  const details = [
    { label: "Versão", value: version },
    { label: "Data", value: scheduleDate },
    { label: "Horário", value: start },
    {
      label: "Profissional",
      value: `${professional?.id ?? "—"} — ${professional?.name ?? "—"}`,
    },
    { label: "Especialidade", value: specialty?.name ?? "—" },
    {
      label: "Unidade",
      value: `${unit?.id ?? "—"} — ${unit?.name ?? "—"}`,
    },
    { label: "Sala", value: room?.name ?? "—" },
  ];

  modalDetails.innerHTML = details
    .map(
      ({ label, value }) => `
        <div>
          <dt>${label}</dt>
          <dd>${value}</dd>
        </div>
      `,
    )
    .join("");

  modal.classList.remove("hidden");
}
const API_ROUTES = {
  v1: "/v1/appoints/available-schedule",
  v2: "/v2/appoints/available-schedule",
};

const PROFESSIONALS = [
  { id: "2684", name: "Dr(a). Pat Duarte" },
  { id: "512", name: "Dr. Ícaro Menezes" },
  { id: "782", name: "Dr(a). Helena Faria" },
  { id: "903", name: "Dr. André Ribeiro" },
];

const UNITS = [
  { id: "901", name: "Clínica Central" },
  { id: "905", name: "Unidade Bela Vista" },
  { id: "910", name: "Centro Norte" },
  { id: "915", name: "Hub Telemedicina" },
];

const VERSION_META = {
  v1: {
    label: "API v1 — Python (FastAPI)",
    professionals: PROFESSIONALS,
    units: UNITS,
  },
  v2: {
    label: "API v2 — Go (net/http)",
    professionals: PROFESSIONALS,
    units: UNITS,
  },
};

const DISPLAY_CARD_COUNT = 2;
const MIN_DAYS_WINDOW = 15;
const MAX_CACHE_WINDOW_DAYS = 120;

const versionSelect = document.querySelector("#version-select");
const refreshButton = document.querySelector("#refresh-btn");
const cardsWrapper = document.querySelector("#cards-wrapper");
const emptyState = document.querySelector("#empty-state");
const skeleton = document.querySelector("#skeleton");
const statusBanner = document.querySelector("#status-banner");
const selectionSummary = document.querySelector("#selection-summary");
const cardTemplate = document.querySelector("#card-template");
const professionalSelect = document.querySelector("#input-professional");
const unitSelect = document.querySelector("#input-unit");
const daysInput = document.querySelector("#input-days");
const calendarGrid = document.querySelector("#calendar-grid");
const calendarMonthLabel = document.querySelector("#calendar-month");
const calendarPrev = document.querySelector("#calendar-prev");
const calendarNext = document.querySelector("#calendar-next");
const modal = document.querySelector("#booking-modal");
const modalDetails = document.querySelector("#booking-details");
const modalCloseButtons = document.querySelectorAll("[data-modal-close]");

document.querySelector("#year").textContent = new Date().getFullYear();

let scheduleData = [];
let availableDates = [];
let availableDateObjs = [];
let selectedDate = null;
let currentMonth = new Date();
let earliestMonth = null;
let latestMonth = null;
const scheduleCache = new Map();

const WEEK_DAYS = ["Seg", "Ter", "Qua", "Qui", "Sex", "Sáb", "Dom"];

function formatDateLabel(dateStr) {
  const [year, month, day] = dateStr.split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  return date.toLocaleDateString("pt-BR", {
    weekday: "long",
    day: "2-digit",
    month: "long",
    timeZone: "UTC",
  });
}

function formatShortDate(dateStr) {
  if (!dateStr) return "—";
  const [year, month, day] = dateStr.split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  return date.toLocaleDateString("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    timeZone: "UTC",
  });
}

function toUtcDate(dateStr) {
  if (!dateStr) return null;
  const [year, month, day] = dateStr.split("-").map(Number);
  return new Date(Date.UTC(year, month - 1, day));
}

function getSequentialDates(startDateStr, count = DISPLAY_CARD_COUNT) {
  const base = toUtcDate(startDateStr);
  if (!base) return [];
  const dates = [];
  for (let i = 0; i < count; i += 1) {
    const next = new Date(base);
    next.setUTCDate(base.getUTCDate() + i);
    dates.push(next.toISOString().slice(0, 10));
  }
  return dates;
}

function highlightStatus(message, tone = "success") {
  statusBanner.textContent = message;
  statusBanner.classList.remove("hidden", "status-error", "status-success");
  statusBanner.classList.add(tone === "success" ? "status-success" : "status-error");
}

function getTodayIso() {
  const now = new Date();
  const utcToday = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  return utcToday.toISOString().slice(0, 10);
}

function pruneScheduleCache(todayIso) {
  const todayDate = toUtcDate(todayIso);
  const maxDate = new Date(todayDate);
  maxDate.setUTCDate(maxDate.getUTCDate() + MAX_CACHE_WINDOW_DAYS);
  const maxIso = maxDate.toISOString().slice(0, 10);

  for (const key of Array.from(scheduleCache.keys())) {
    if (key < todayIso || key > maxIso) {
      scheduleCache.delete(key);
    }
  }
}

function rebuildAvailableDates(todayIso) {
  const sorted = Array.from(scheduleCache.keys())
    .filter((date) => date >= todayIso)
    .sort();
  availableDates = sorted;
  availableDateObjs = sorted.map((dateStr) => toUtcDate(dateStr)).filter(Boolean);
}

function toggleSkeleton(show) {
  skeleton.classList.toggle("hidden", !show);
  cardsWrapper.classList.toggle("opacity-30", show);
}

function populateSelect(selectEl, items = []) {
  selectEl.innerHTML = "";
  items.forEach((item) => {
    const option = document.createElement("option");
    option.value = String(item.id);
    option.textContent = item.name;
    selectEl.appendChild(option);
  });
}

function ensureOption(selectEl, id, label) {
  if (!id) return;
  const idStr = String(id);
  const exists = Array.from(selectEl.options).some((opt) => opt.value === idStr);
  if (!exists) {
    const option = document.createElement("option");
    option.value = idStr;
    option.textContent = label || idStr;
    selectEl.appendChild(option);
  }
}

function setDefaultsForVersion(version) {
  const meta = VERSION_META[version] || VERSION_META.v1;
  populateSelect(professionalSelect, meta.professionals);
  populateSelect(unitSelect, meta.units);
  if (meta.professionals?.length) {
    professionalSelect.value = String(meta.professionals[0].id);
  }
  if (meta.units?.length) {
    unitSelect.value = String(meta.units[0].id);
  }
}

function alignDaysValue() {
  const expected = String(MIN_DAYS_WINDOW);
  if (daysInput.value !== expected) {
    daysInput.value = expected;
  }
  return daysInput.value;
}

async function fetchSchedules(version, options = {}) {
  const { startDate } = options;
  const daysValue = alignDaysValue();
  const params = new URLSearchParams({
    professional_id: professionalSelect.value || "2684",
    unit_id: unitSelect.value || "901",
    days: daysValue,
  });
  if (startDate) {
    params.set("start_date", startDate);
  }

  const response = await fetch(`${API_ROUTES[version]}?${params.toString()}`, {
    headers: { "Content-Type": "application/json" },
  });

  if (!response.ok) {
    throw new Error(`Erro ${response.status}`);
  }

  return response.json();
}

function renderSelectionSummary(version, payload) {
  if (!selectionSummary) {
    return;
  }
  const filters = payload?.filters;
  if (!filters) {
    selectionSummary.classList.add("hidden");
    return;
  }

  const professionalName =
    professionalSelect.options[professionalSelect.selectedIndex]?.text ||
    filters.professional_id ||
    professionalSelect.value;
  const unitName =
    unitSelect.options[unitSelect.selectedIndex]?.text || filters.unit_id || unitSelect.value;
  const appliedStart = filters.start_date_applied || availableDates[0];
  const generatedAt = filters.generated_at
    ? new Date(filters.generated_at).toLocaleTimeString("pt-BR")
    : new Date().toLocaleTimeString("pt-BR");

  selectionSummary.textContent = [
    VERSION_META[version]?.label ?? version.toUpperCase(),
    `Profissional: ${professionalSelect.value} – ${professionalName}`,
    `Unidade: ${unitSelect.value} – ${unitName}`,
    `Início: ${formatShortDate(appliedStart)}`,
    `Dias retornados: ${filters.days_returned ?? MIN_DAYS_WINDOW}`,
    `Gerado às ${generatedAt}`,
  ]
    .filter(Boolean)
    .join(" • ");
  selectionSummary.classList.remove("hidden");
}

function renderCalendar() {
  if (!availableDates.length) {
    calendarMonthLabel.textContent = "Sem datas disponíveis";
    calendarGrid.innerHTML =
      "<div class='calendar-empty'>Nenhuma data futura disponível</div>";
    calendarPrev.disabled = true;
    calendarNext.disabled = true;
    return;
  }

  calendarGrid.innerHTML = "";

  WEEK_DAYS.forEach((day) => {
    const label = document.createElement("div");
    label.className = "day-label";
    label.textContent = day;
    calendarGrid.appendChild(label);
  });

  if (earliestMonth && currentMonth < earliestMonth) {
    currentMonth = new Date(earliestMonth);
  }
  if (latestMonth && currentMonth > latestMonth) {
    currentMonth = new Date(latestMonth);
  }

  const currentYear = currentMonth.getUTCFullYear();
  const currentMon = currentMonth.getUTCMonth();
  const firstDay = new Date(Date.UTC(currentYear, currentMon, 1));
  const daysInMonth = new Date(Date.UTC(currentYear, currentMon + 1, 0)).getUTCDate();

  const leadingBlanks = (firstDay.getUTCDay() + 6) % 7; // Monday start

  for (let i = 0; i < leadingBlanks; i++) {
    const blank = document.createElement("div");
    blank.className = "calendar-day calendar-day--muted";
    calendarGrid.appendChild(blank);
  }

  const availableSet = new Set(availableDates);
  const todayUtc = new Date(Date.UTC(new Date().getFullYear(), new Date().getMonth(), new Date().getDate()));

  for (let day = 1; day <= daysInMonth; day++) {
    const date = new Date(Date.UTC(currentYear, currentMon, day));
    const dateStr = date.toISOString().slice(0, 10);
    const button = document.createElement("button");
    button.className = "calendar-day";
    button.textContent = String(day).padStart(2, "0");

    if (availableSet.has(dateStr) && date >= todayUtc) {
      button.classList.add("calendar-day--available");
      if (selectedDate === dateStr) {
        button.classList.add("calendar-day--selected");
      }
      button.addEventListener("click", () => handleDateSelection(dateStr));
    } else {
      button.classList.add("calendar-day--muted");
      button.disabled = true;
    }

    calendarGrid.appendChild(button);
  }

  calendarPrev.disabled = Boolean(
    earliestMonth && currentMonth.getTime() <= earliestMonth.getTime(),
  );
  calendarNext.disabled = Boolean(latestMonth && currentMonth.getTime() >= latestMonth.getTime());

  calendarMonthLabel.textContent = new Date(
    Date.UTC(currentMonth.getUTCFullYear(), currentMonth.getUTCMonth(), 1),
  ).toLocaleDateString("pt-BR", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });
}

function handleDateSelection(dateStr) {
  if (!dateStr) return;
  selectedDate = dateStr;
  renderCalendar();
  refresh({ startDate: dateStr });
}

function renderSchedulesForDate(dateStr) {
  cardsWrapper.innerHTML = "";

  if (!dateStr) {
    emptyState.classList.remove("hidden");
    return;
  }

  const targetDates = getSequentialDates(dateStr, DISPLAY_CARD_COUNT);
  if (!targetDates.length) {
    emptyState.classList.remove("hidden");
    return;
  }

  let rendered = 0;

  targetDates.forEach((targetDate) => {
    const schedule = scheduleCache.get(targetDate);
    const node = cardTemplate.content.cloneNode(true);
    const cardElement = node.querySelector(".schedule-card");

    const displayDate = schedule?.date ?? targetDate;
    node.querySelector(".schedule-card__title").textContent = schedule
      ? `Agenda – ${schedule.specialty?.name ?? "Especialidade"}`
      : "Agenda indisponível";
    node.querySelector(".schedule-card__subtitle").textContent = formatDateLabel(displayDate);

    const badgeDate = toUtcDate(displayDate);
    node.querySelector(".badge").textContent = badgeDate
      ? badgeDate.toLocaleDateString("pt-BR", {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
          timeZone: "UTC",
        })
      : formatShortDate(displayDate);

    node.querySelector("[data-professional]").textContent =
      schedule?.professional?.name ?? "—";
    node.querySelector("[data-specialty]").textContent = schedule?.specialty?.name ?? "—";
    node.querySelector("[data-unit]").textContent = schedule?.unit?.name ?? "—";
    node.querySelector("[data-room]").textContent = schedule?.room?.name ?? "—";

    const slotsWrapper = node.querySelector("[data-slots]");
    slotsWrapper.innerHTML = "";
    if (!schedule?.slots?.length) {
      const empty = document.createElement("div");
      empty.className = "text-sm text-slate-500";
      empty.textContent = "Nenhum horário disponível para esta data.";
      slotsWrapper.appendChild(empty);
      if (!schedule) {
        cardElement.classList.add("schedule-card--placeholder");
      }
    } else {
      schedule.slots.forEach((slot) => {
        const slotEl = document.createElement("button");
        slotEl.className = slot.available ? "slot slot--available" : "slot slot--busy";
        slotEl.textContent = slot.start;
        slotEl.disabled = !slot.available;
        if (slot.available) {
          slotEl.addEventListener("click", () => {
            const versionLabel =
              VERSION_META[versionSelect.value]?.label ?? versionSelect.value.toUpperCase();
            const professionalName =
              professionalSelect.options[professionalSelect.selectedIndex]?.text ||
              schedule.professional?.name ||
              "-";
            const unitName =
              unitSelect.options[unitSelect.selectedIndex]?.text ||
              schedule.unit?.name ||
              "-";

            openBookingModal({
              version: versionLabel,
              date: schedule.date,
              start: slot.start,
              professional: {
                id: schedule.professional?.id,
                name: professionalName,
              },
              specialty: schedule.specialty,
              unit: {
                id: schedule.unit?.id,
                name: unitName,
              },
              room: schedule.room,
            });
          });
        }
        slotsWrapper.appendChild(slotEl);
      });
    }

    cardsWrapper.appendChild(node);
    rendered += 1;
  });

  if (!rendered) {
    emptyState.classList.remove("hidden");
    return;
  }
  emptyState.classList.add("hidden");
}

async function refresh(options = {}) {
  const { startDate, preserveSelection = false } = options;
  const version = versionSelect.value;
  const startLabel = startDate ? ` a partir de ${formatShortDate(startDate)}` : "";
  toggleSkeleton(true);
  highlightStatus(
    `Consultando ${VERSION_META[version]?.label ?? version.toUpperCase()}${startLabel}…`,
    "success",
  );

  try {
    const payload = await fetchSchedules(version, { startDate });
    const entries = Array.isArray(payload?.response) ? payload.response : [];
    scheduleData = entries;

    const todayIso = getTodayIso();

    entries.forEach((entry) => {
      if (entry?.date) {
        scheduleCache.set(entry.date, entry);
      }
    });

    pruneScheduleCache(todayIso);
    rebuildAvailableDates(todayIso);

    if (!availableDates.length) {
      selectedDate = null;
      renderCalendar();
      renderSchedulesForDate("");
      highlightStatus("Nenhuma data disponível para os filtros informados.", "error");
      return;
    }

    const todayDateObj = toUtcDate(todayIso);
    const nowMonth = new Date(
      Date.UTC(todayDateObj.getUTCFullYear(), todayDateObj.getUTCMonth(), 1),
    );

    earliestMonth = new Date(
      Date.UTC(availableDateObjs[0].getUTCFullYear(), availableDateObjs[0].getUTCMonth(), 1),
    );
    if (earliestMonth < nowMonth) {
      earliestMonth = new Date(nowMonth);
    }
    latestMonth = new Date(
      Date.UTC(
        availableDateObjs[availableDateObjs.length - 1].getUTCFullYear(),
        availableDateObjs[availableDateObjs.length - 1].getUTCMonth(),
        1,
      ),
    );

    const filters = payload?.filters ?? {};
    const candidateStart =
      (filters.start_date_applied && availableDates.includes(filters.start_date_applied)
        ? filters.start_date_applied
        : undefined) ||
      (startDate && availableDates.includes(startDate) ? startDate : undefined) ||
      availableDates[0];

    if (preserveSelection && selectedDate && availableDates.includes(selectedDate)) {
      // keep current selection
    } else {
      selectedDate = candidateStart;
    }

    if (!selectedDate || !availableDates.includes(selectedDate)) {
      selectedDate = availableDates[0];
    }

    const defaultsSchedule =
      scheduleCache.get(selectedDate) ||
      (candidateStart && scheduleCache.get(candidateStart)) ||
      entries.find(Boolean) ||
      (availableDates.length ? scheduleCache.get(availableDates[0]) : null);

    const professionalId =
      filters.professional_id ?? defaultsSchedule?.professional?.id ?? professionalSelect.value;
    const professionalName = defaultsSchedule?.professional?.name;
    if (professionalId !== undefined && professionalId !== null && professionalId !== "") {
      ensureOption(professionalSelect, professionalId, professionalName);
      professionalSelect.value = String(professionalId);
    }

    const unitId = filters.unit_id ?? defaultsSchedule?.unit?.id ?? unitSelect.value;
    const unitName = defaultsSchedule?.unit?.name;
    if (unitId !== undefined && unitId !== null && unitId !== "") {
      ensureOption(unitSelect, unitId, unitName);
      unitSelect.value = String(unitId);
    }

    const selectedDateObj = toUtcDate(selectedDate) ?? availableDateObjs[0];
    currentMonth = new Date(
      Date.UTC(selectedDateObj.getUTCFullYear(), selectedDateObj.getUTCMonth(), 1),
    );
    if (currentMonth < earliestMonth) {
      currentMonth = new Date(earliestMonth);
    }
    if (currentMonth > latestMonth) {
      currentMonth = new Date(latestMonth);
    }

    renderCalendar();
    renderSchedulesForDate(selectedDate);
    renderSelectionSummary(version, payload);
    const appliedLabel = formatShortDate(candidateStart);
    highlightStatus(
      `Versão ${version.toUpperCase()} sincronizada às ${new Date().toLocaleTimeString("pt-BR")} (início ${appliedLabel}).`,
      "success",
    );
  } catch (error) {
    console.error(error);
    scheduleData = [];
    availableDates = [];
    renderCalendar();
    renderSchedulesForDate("");
    highlightStatus("Não foi possível carregar os dados. Verifique as APIs v1/v2.", "error");
  } finally {
    toggleSkeleton(false);
  }
}

modalCloseButtons.forEach((button) => {
  button.addEventListener("click", () => modal.classList.add("hidden"));
});

modal.addEventListener("click", (event) => {
  if (event.target === modal || event.target.dataset.modalClose !== undefined) {
    modal.classList.add("hidden");
  }
});

calendarPrev.addEventListener("click", () => {
  if (calendarPrev.disabled || !earliestMonth) return;
  const prev = new Date(Date.UTC(currentMonth.getUTCFullYear(), currentMonth.getUTCMonth() - 1, 1));
  if (prev < earliestMonth) {
    currentMonth = new Date(earliestMonth);
  } else {
    currentMonth = prev;
  }
  renderCalendar();
});

calendarNext.addEventListener("click", () => {
  if (calendarNext.disabled) return;
  const next = new Date(Date.UTC(currentMonth.getUTCFullYear(), currentMonth.getUTCMonth() + 1, 1));
  if (latestMonth && next > latestMonth) {
    currentMonth = new Date(latestMonth);
  } else {
    currentMonth = next;
  }
  renderCalendar();
});

refreshButton.addEventListener("click", () => {
  const start = selectedDate ?? availableDates[0];
  refresh({ startDate: start, preserveSelection: true });
});

versionSelect.addEventListener("change", () => {
  setDefaultsForVersion(versionSelect.value);
  const start = selectedDate ?? undefined;
  refresh({ startDate: start });
});

setDefaultsForVersion(versionSelect.value);
refresh();