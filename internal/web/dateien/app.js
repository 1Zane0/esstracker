/* Esstracker — Oberflaeche.
 *
 * Derselbe Grundsatz wie in der Tasklist nebenan: die Oberflaeche rechnet
 * nichts aus. Jede Anfrage an den Dienst — auch jede aendernde — antwortet mit
 * dem vollstaendigen Stand, und danach wird alles neu gezeichnet. Es gibt
 * deshalb keinen zweiten Ort, an dem eine Tagessumme oder eine Rasterstufe
 * entsteht, und damit auch keinen, an dem sie auseinanderlaufen koennen.
 *
 * Gebaut wird mit createElement statt mit Zeichenketten: Produktnamen kommen
 * aus einer fremden Datenbank ueber ein Handy herein, und textContent kann man
 * nicht versehentlich als HTML ausfuehren lassen.
 */
'use strict';

const $ = (s, w = document) => w.querySelector(s);
let stand = null;
let seite = 'heute';
let uhr = null;      // laeuft nur, solange ein Kopplungsfenster offen ist

/* ---------- Bausteine ---------- */

function el(tag, klasse, text) {
  const k = document.createElement(tag);
  if (klasse) k.className = klasse;
  if (text !== undefined && text !== null) k.textContent = text;
  return k;
}

function bild(id, groesse = 16) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('width', groesse);
  svg.setAttribute('height', groesse);
  const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
  use.setAttribute('href', '#' + id);
  svg.appendChild(use);
  return svg;
}

function knopf(symbol, titel, tat) {
  const b = el('button', 'knopf leise');
  b.type = 'button';
  b.title = titel;
  b.setAttribute('aria-label', titel);
  b.appendChild(bild(symbol, 14));
  b.addEventListener('click', (e) => { e.stopPropagation(); tat(); });
  return b;
}

let meldungUhr = null;

// melde zeigt einen Satz am unteren Rand. Ein Fehler darf hier nicht im
// Terminal landen — es gibt keins.
function melde(text, dauer = 4000) {
  let m = $('.meldung');
  if (!m) { m = el('div', 'meldung'); document.body.appendChild(m); }
  m.textContent = text;
  clearTimeout(meldungUhr);
  meldungUhr = setTimeout(() => m.remove(), dauer);
}

/* ---------- Zahlen ---------- */

const zahl = (n) => Math.round(n || 0).toLocaleString('de-DE');

// gramm rundet erst ab einer Nachkommastelle, damit 0,8 g Salz nicht zu 1 g
// wird und 27 g Fett nicht zu 27,0 g.
function gramm(n) {
  const w = n || 0;
  if (w >= 10) return Math.round(w).toLocaleString('de-DE');
  return w.toLocaleString('de-DE', { maximumFractionDigits: 1 });
}

const wochentage = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'];
const monate = ['Jan', 'Feb', 'Mär', 'Apr', 'Mai', 'Jun', 'Jul', 'Aug', 'Sep', 'Okt', 'Nov', 'Dez'];

// Das Datum kommt als JJJJ-MM-TT. Als lokale Zeit auswerten, nicht ueber
// new Date(text) — das liest ISO-Daten als UTC und verschiebt sie westlich von
// Greenwich um einen Tag.
function tagAlsDatum(tag) {
  const [j, m, t] = tag.split('-').map(Number);
  return new Date(j, m - 1, t);
}

function tagLang(tag) {
  const d = tagAlsDatum(tag);
  return wochentage[d.getDay()] + ', ' + d.getDate() + '. ' + monate[d.getMonth()] + ' ' + d.getFullYear();
}

function tagKurz(tag) {
  const d = tagAlsDatum(tag);
  return d.getDate() + '.' + (d.getMonth() + 1) + '.';
}

const mahlzeitenNamen = {
  fruehstueck: 'Frühstück', mittag: 'Mittag', abend: 'Abend', snack: 'Zwischendurch',
};
const mahlzeitenFolge = ['fruehstueck', 'mittag', 'abend', 'snack'];

/* ---------- Reden mit dem Dienst ---------- */

async function hole(pfad, art = 'GET', rumpf) {
  const antwort = await fetch(pfad, {
    method: art,
    headers: rumpf ? { 'Content-Type': 'application/json' } : undefined,
    body: rumpf ? JSON.stringify(rumpf) : undefined,
  });
  const text = await antwort.text();
  let daten = null;
  try { daten = text ? JSON.parse(text) : null; } catch (_) { /* kein JSON */ }
  if (!antwort.ok) throw new Error((daten && daten.fehler) || 'Fehler ' + antwort.status);
  return daten;
}

async function laden(pfad = '/api/stand', art = 'GET', rumpf) {
  try {
    stand = await hole(pfad, art, rumpf);
    zeichne();
  } catch (fehler) {
    melde(fehler.message);
  }
}

/* ---------- Zeichnen ---------- */

function zeichne() {
  if (!stand) return;
  puls();
  menue();
  if (seite === 'heute') heute();
  if (seite === 'verlauf') verlauf();
  if (seite === 'produkte') produkte();
  if (seite === 'handy') handy();
  $('#fuss-datum').textContent = tagLang(stand.heute);
}

function puls() {
  const z = stand.zahlen;
  $('#puls-ist').textContent = zahl(z.kcal);
  $('#puls-soll').textContent = 'von ' + zahl(z.ziel) + ' kcal';
  $('#puls-balken').style.width = Math.min(100, z.prozent) + '%';

  const rest = $('#puls-rest');
  rest.className = 'serie';
  if (z.eintraege === 0) {
    rest.textContent = 'noch nichts heute';
    rest.classList.add('kalt');
  } else if (z.rest >= 0) {
    rest.replaceChildren(el('b', null, zahl(z.rest)), document.createTextNode(' kcal übrig'));
  } else {
    rest.replaceChildren(el('b', null, zahl(-z.rest)), document.createTextNode(' kcal darüber'));
  }
}

function menue() {
  const z = stand.zahlen;
  setzeZahl('#zahl-heute', z.eintraege);
  setzeZahl('#zahl-produkte', stand.produkte.length);
  setzeZahl('#zahl-handy', stand.geraete.length);
}

function setzeZahl(wahl, n) {
  const k = $(wahl);
  k.textContent = n > 0 ? n : '';
  k.classList.toggle('leer', !(n > 0));
}

/* ---------- Heute ---------- */

// tagVon sucht einen Tag im Stand. Der heutige fehlt, solange nichts
// eingetragen ist — das ist der Normalfall am Morgen, kein Sonderfall.
function tagVon(datum) {
  return stand.tage.find((t) => t.tag === datum) || null;
}

function heute() {
  const t = tagVon(stand.heute);
  const z = stand.zahlen;

  $('#heute-zusatz').textContent = tagLang(stand.heute);
  $('#heute-anzahl').textContent = z.eintraege === 0 ? '' :
    z.eintraege + (z.eintraege === 1 ? ' Eintrag' : ' Einträge');

  naehrwerte($('#heute-naehrwerte'), z.naehrwerte, z.kcal, z.ziel);

  const liste = $('#heute-liste');
  if (!t) {
    liste.replaceChildren(el('div', 'leer', 'Heute ist noch nichts angekommen. Eingetragen wird am Handy.'));
    return;
  }
  liste.replaceChildren(...mahlzeitgruppen(t));
}

// naehrwerte zeichnet vier Zahlen. Die Balken zeigen den Anteil an einem
// Richtwert, der sich aus dem Tagesziel ergibt — sonst haette „88 g Fett"
// keinen Bezug, gegen den man es liest.
function naehrwerte(wohin, n, kcal, ziel) {
  // Die Verteilung folgt den Referenzwerten der Naehrwerttabelle: rund 30 %
  // der Energie aus Fett, 50 % aus Kohlenhydraten, 20 % aus Eiweiss.
  const richt = {
    fett: (ziel * 0.30) / 9,
    kohlenhydrate: (ziel * 0.50) / 4,
    eiweiss: (ziel * 0.20) / 4,
  };
  const felder = [
    ['Energie', zahl(kcal), 'kcal', kcal, ziel],
    ['Fett', gramm(n.fett), 'g', n.fett, richt.fett],
    ['Kohlenhydrate', gramm(n.kohlenhydrate), 'g', n.kohlenhydrate, richt.kohlenhydrate],
    ['Eiweiß', gramm(n.eiweiss), 'g', n.eiweiss, richt.eiweiss],
  ];
  wohin.replaceChildren(...felder.map(([name, wert, einheit, ist, soll]) => {
    const k = el('div', 'naehrwert');
    k.appendChild(el('dt', null, name));
    const dd = el('dd', null, wert);
    dd.appendChild(el('small', null, ' ' + einheit));
    k.appendChild(dd);
    const b = el('div', 'balken');
    const i = el('i');
    i.style.width = Math.min(100, soll > 0 ? (ist / soll) * 100 : 0) + '%';
    b.appendChild(i);
    k.appendChild(b);
    return k;
  }));
}

// mahlzeitgruppen buendelt die Zeilen eines Tages nach Mahlzeit. Leere
// Mahlzeiten fallen weg: eine Ueberschrift „Mittag" ohne Zeile darunter
// behauptet, es fehle etwas.
function mahlzeitgruppen(t) {
  const gruppen = [];
  for (const art of mahlzeitenFolge) {
    const zeilen = t.zeilen.filter((z) => z.mahlzeit === art);
    if (!zeilen.length) continue;

    const g = el('div', 'mahlzeit');
    const h = el('h3', null, mahlzeitenNamen[art]);
    const summe = zeilen.reduce((s, z) => s + z.kcal, 0);
    h.appendChild(el('span', 'summe', zahl(summe) + ' kcal'));
    g.appendChild(h);
    for (const z of zeilen) g.appendChild(eintragszeile(z));
    gruppen.push(g);
  }
  return gruppen;
}

function eintragszeile(z) {
  const r = el('div', 'eintrag');
  r.appendChild(el('div', 'uhr', z.uhr));

  const was = el('div', 'was');
  was.appendChild(el('b', null, z.name));
  // Unter dem Namen steht, woraus die Zahl entstanden ist: Menge mal Wert je
  // 100 g. Wer eine Kalorienzahl anzweifelt, sieht hier sofort, ob die Menge
  // schuld ist oder die Datenbank.
  const teile = [];
  if (z.marke) teile.push(z.marke);
  teile.push(gramm(z.menge) + ' ' + z.einheit);
  if (z.je100 && z.je100.kcal) teile.push(zahl(z.je100.kcal) + ' kcal/100 ' + z.einheit);
  was.appendChild(el('span', null, teile.join(' · ')));
  r.appendChild(was);

  const k = el('div', 'kcal');
  k.appendChild(el('b', null, zahl(z.kcal)));
  k.appendChild(el('span', null, 'kcal'));
  r.appendChild(k);

  const werkzeuge = el('div', 'werkzeuge');
  werkzeuge.appendChild(knopf('i-weg', 'Eintrag löschen', () => {
    laden('/api/eintrag/' + encodeURIComponent(z.id), 'DELETE');
    melde('„' + z.name + '" gelöscht — am Handy bleibt der Eintrag stehen.');
  }));
  r.appendChild(werkzeuge);

  const anteil = el('div', 'anteil');
  const i = el('i');
  i.style.width = Math.min(100, stand.ziel > 0 ? (z.kcal / stand.ziel) * 100 : 0) + '%';
  anteil.appendChild(i);
  r.appendChild(anteil);

  return r;
}

/* ---------- Verlauf ---------- */

function verlauf() {
  const z = stand.zahlen;
  const kacheln = [
    ['Heute', zahl(z.kcal) + ' kcal', z.prozent + ' % vom Ziel'],
    ['Schnitt', zahl(z.schnitt7) + ' kcal', 'die letzten 7 Tage mit Einträgen'],
    ['Tage', zahl(z.tage), z.tage === 1 ? 'Tag mit Einträgen' : 'Tage mit Einträgen'],
    ['Einträge', zahl(stand.tage.reduce((s, t) => s + t.zeilen.length, 0)), 'insgesamt angekommen'],
  ];
  $('#verlauf-kacheln').replaceChildren(...kacheln.map(([name, wert, unten]) => {
    const k = el('div', 'kachel');
    k.appendChild(el('dt', null, name));
    k.appendChild(el('dd', null, wert));
    k.appendChild(el('small', null, unten));
    return k;
  }));

  woche();
  raster();

  // Der heutige Tag steht schon auf der ersten Seite — hier waere er doppelt.
  const aeltere = stand.tage.filter((t) => t.tag !== stand.heute);
  const wohin = $('#verlauf-tage');
  if (!aeltere.length) {
    wohin.replaceChildren(el('div', 'leer', 'Noch keine älteren Tage.'));
    return;
  }
  wohin.replaceChildren(...aeltere.map(tageszeile));
}

function tageszeile(t) {
  const r = el('div', 'produkt');
  const was = el('div', 'was');
  was.appendChild(el('b', null, tagLang(t.tag)));
  was.appendChild(el('span', null,
    t.zeilen.length + (t.zeilen.length === 1 ? ' Eintrag · ' : ' Einträge · ') +
    'Fett ' + gramm(t.naehrwerte.fett) + ' g · ' +
    'Kohlenhydrate ' + gramm(t.naehrwerte.kohlenhydrate) + ' g · ' +
    'Eiweiß ' + gramm(t.naehrwerte.eiweiss) + ' g'));
  r.appendChild(was);

  const werte = el('div', 'werte');
  werte.appendChild(wertfeld(zahl(t.kcal), 'kcal'));
  werte.appendChild(wertfeld(t.prozent + ' %', 'vom Ziel'));
  r.appendChild(werte);
  return r;
}

function wertfeld(gross, klein) {
  const d = el('div');
  d.appendChild(el('b', null, gross));
  d.appendChild(document.createTextNode(klein));
  return d;
}

function woche() {
  // Die Achse geht bewusst ueber das Ziel hinaus. Reichte sie genau bis
  // dorthin, laege die Ziellinie an Tagen unter dem Ziel — also fast immer —
  // am oberen Rand und wuerde weggeschnitten. Sichtbar ist sie nur dann etwas
  // wert, wenn sie auch dann dasteht, wenn man sie nicht reisst.
  const hoechst = Math.max(stand.ziel * 1.15, ...stand.woche.map((p) => p.kcal));
  $('#woche-zusatz').textContent = 'Strich: Ziel ' + zahl(stand.ziel) + ' kcal';

  $('#woche').replaceChildren(...stand.woche.map((p) => {
    const d = el('div', 'wochentag' + (p.heute ? ' heute' : ''));
    const saeule = el('div', 'saeule');
    const i = el('i');
    i.style.height = hoechst > 0 ? (p.kcal / hoechst) * 100 + '%' : '0';
    saeule.appendChild(i);
    const linie = el('div', 'ziellinie');
    linie.style.bottom = hoechst > 0 ? (stand.ziel / hoechst) * 100 + '%' : '0';
    saeule.appendChild(linie);
    d.appendChild(saeule);
    d.appendChild(el('div', 'zahl', p.kcal ? zahl(p.kcal) : '—'));
    d.appendChild(el('div', 'tag', wochentage[tagAlsDatum(p.tag).getDay()] + ' ' + tagKurz(p.tag)));
    return d;
  }));
}

function raster() {
  const felder = stand.raster;
  const mitEintrag = felder.filter((f) => f.kcal > 0).length;
  $('#raster-zusatz').textContent = mitEintrag + (mitEintrag === 1 ? ' Tag getrackt' : ' Tage getrackt');

  $('#raster').replaceChildren(...felder.map((f) => {
    const i = el('i');
    if (f.stufe > 0) i.classList.add('s' + f.stufe);
    if (f.tag === stand.heute) i.classList.add('heute');
    i.title = tagLang(f.tag) + (f.kcal > 0 ? ' — ' + zahl(f.kcal) + ' kcal' : ' — nichts eingetragen');
    return i;
  }));

  // Die Monatsleiste: je Spalte eine Zelle, beschriftet wird nur die erste
  // Spalte eines Monats. Anders bekommt man die Beschriftung nicht ueber die
  // richtige Woche, weil die Spalten unterschiedlich viele Tage je Monat haben.
  const spalten = Math.ceil(felder.length / 7);
  const leiste = $('#monate');
  leiste.style.gridTemplateColumns = 'repeat(' + spalten + ', 14px)';
  const zellen = [];
  let letzter = -1;
  for (let s = 0; s < spalten; s++) {
    const f = felder[s * 7];
    const m = f ? tagAlsDatum(f.tag).getMonth() : -1;
    // Erst ab der zweiten Woche eines Monats beschriften: sonst steht das
    // Kuerzel ueber einer Spalte, die fast ganz zum Vormonat gehoert.
    const tagImMonat = f ? tagAlsDatum(f.tag).getDate() : 99;
    zellen.push(el('span', null, m !== letzter && tagImMonat <= 7 ? (letzter = m, monate[m]) : ''));
  }
  leiste.replaceChildren(...zellen);

  // Ans rechte Ende rollen: dort steht der heutige Tag. Ein Jahresraster, das
  // links beim Vorjahr beginnt, zeigt beim Aufschlagen die Spalten, um die es
  // gerade am wenigsten geht. Nur rollen, wenn der Mensch nicht selbst gerollt
  // hat — sonst springt es ihm bei jedem Nachladen unter der Hand weg.
  const huelle = $('.rasterhuelle');
  if (!huelle.dataset.gerollt) {
    huelle.scrollLeft = huelle.scrollWidth;
    huelle.addEventListener('scroll', () => { huelle.dataset.gerollt = '1'; }, { once: true });
  }
}

/* ---------- Produkte ---------- */

function produkte() {
  const wohin = $('#produkte-liste');
  if (!stand.produkte.length) {
    wohin.replaceChildren(el('div', 'leer', 'Noch keine Produkte angekommen.'));
    return;
  }
  wohin.replaceChildren(...stand.produkte.map((p, i) => {
    const r = el('div', 'produkt');
    r.appendChild(el('div', 'nummer', i + 1));

    const was = el('div', 'was');
    was.appendChild(el('b', null, p.name));
    const teile = [];
    if (p.marke) teile.push(p.marke);
    if (p.kcal100) teile.push(zahl(p.kcal100) + ' kcal je 100 ' + (p.einheit || 'g'));
    if (p.barcode) teile.push(p.barcode);
    teile.push('zuletzt ' + tagKurz(p.zuletzt));
    was.appendChild(el('span', null, teile.join(' · ')));
    r.appendChild(was);

    const werte = el('div', 'werte');
    werte.appendChild(wertfeld(zahl(p.anzahl), p.anzahl === 1 ? 'mal' : 'mal'));
    werte.appendChild(wertfeld(gramm(p.menge), 'insg. ' + (p.einheit || 'g')));
    r.appendChild(werte);
    return r;
  }));
}

/* ---------- Handy ---------- */

function handy() {
  kopplung();

  $('#geraete-zusatz').textContent = stand.geraete.length
    ? stand.geraete.length + (stand.geraete.length === 1 ? ' Gerät' : ' Geräte') : '';

  const wohin = $('#geraete-liste');
  if (!stand.geraete.length) {
    wohin.replaceChildren(el('div', 'leer', 'Noch kein Handy gekoppelt.'));
  } else {
    wohin.replaceChildren(...stand.geraete.map(geraetezeile));
  }

  const feld = $('#form-ziel').kcal;
  if (document.activeElement !== feld) feld.value = stand.ziel;
}

function geraetezeile(g) {
  const r = el('div', 'geraet');
  const was = el('div', 'was');
  was.appendChild(el('b', null, g.name));
  was.appendChild(el('span', null, g.id));
  r.appendChild(was);

  const werte = el('div', 'werte');
  werte.appendChild(wertfeld(zahl(g.eintraege), 'Einträge'));
  werte.appendChild(wertfeld(g.zuletzt ? zeitKurz(g.zuletzt) : '—', 'zuletzt'));
  r.appendChild(werte);

  const werkzeuge = el('div', 'werkzeuge');
  werkzeuge.style.opacity = '1';
  werkzeuge.appendChild(knopf('i-weg', 'Gerät trennen', () => {
    laden('/api/geraet/' + encodeURIComponent(g.id), 'DELETE');
    melde('„' + g.name + '" getrennt. Die Einträge bleiben.');
  }));
  r.appendChild(werkzeuge);
  return r;
}

function zeitKurz(rfc) {
  const d = new Date(rfc);
  if (isNaN(d)) return '—';
  const heute = tagAlsDatum(stand.heute);
  const selberTag = d.toDateString() === heute.toDateString();
  return selberTag
    ? d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })
    : d.getDate() + '.' + (d.getMonth() + 1) + '.';
}

function kopplung() {
  const k = stand.kopplung;
  const kasten = $('#kopplung');
  kasten.className = 'kopplung' + (k.offen ? ' offen' : '');
  kasten.replaceChildren();

  const h = el('h2', null);
  h.style.cssText = 'font-size:13.5px;font-weight:600;margin:0;display:flex;align-items:center;gap:8px';
  h.appendChild(bild('i-funk', 15));
  h.appendChild(document.createTextNode(k.offen ? 'Kopplung ist offen' : 'Neues Handy koppeln'));
  kasten.appendChild(h);

  if (k.offen) {
    kasten.appendChild(el('div', 'wort', k.wort));
    kasten.appendChild(el('div', 'rest', 'noch ' + mmss(k.bisSek) + ' — schließt sich, sobald ein Handy durchkommt'));
    const p = el('p', null, 'Am Handy: Einstellungen → „Rechner suchen". Es findet diesen Rechner von allein und zeigt dabei dasselbe Wort.');
    kasten.appendChild(p);
  } else {
    const ol = el('ol');
    for (const schritt of [
      'Handy und Rechner im selben WLAN.',
      'Hier auf „Kopplung öffnen" — das Fenster steht fünf Minuten offen.',
      'Am Handy: Einstellungen → „Rechner suchen".',
      'Das Wort auf beiden Seiten vergleichen, fertig.',
    ]) ol.appendChild(el('li', null, schritt));
    kasten.appendChild(ol);
  }

  const schub = el('div', 'schub');
  if (k.offen) {
    const zu = el('button', 'knopf', 'Kopplung schließen');
    zu.addEventListener('click', () => laden('/api/kopplung/zu', 'POST'));
    schub.appendChild(zu);
  } else {
    const auf = el('button', 'knopf haupt', 'Kopplung öffnen');
    auf.addEventListener('click', () => laden('/api/kopplung/auf', 'POST'));
    schub.appendChild(auf);
  }
  kasten.appendChild(schub);

  // Die Adressen stehen auch dann da, wenn die Suche nicht durchkommt — in
  // manchen Netzen ist der UDP-Rundruf gesperrt, und dann ist die Adresse von
  // Hand der einzige Weg.
  if (k.adressen && k.adressen.length) {
    const hinweis = el('p', null, 'Falls die Suche nicht durchkommt, am Handy von Hand eintragen:');
    hinweis.style.fontSize = '12.5px';
    kasten.appendChild(hinweis);
    const a = el('div', 'adressen');
    for (const adr of k.adressen) a.appendChild(el('code', null, adr + ':' + k.port));
    kasten.appendChild(a);
  }

  // Die Uhr laeuft nur, solange sie etwas anzeigt.
  clearInterval(uhr);
  uhr = null;
  if (k.offen && seite === 'handy') {
    uhr = setInterval(() => laden(), 1000);
  }
}

function mmss(sek) {
  const m = Math.floor(sek / 60);
  const s = sek % 60;
  return m + ':' + String(s).padStart(2, '0');
}

/* ---------- Seiten ---------- */

const titel = { heute: 'Heute', verlauf: 'Verlauf', produkte: 'Produkte', handy: 'Handy' };

function zeigeSeite(neu) {
  seite = neu;
  for (const s of document.querySelectorAll('.seite')) s.classList.toggle('an', s.id === 'seite-' + neu);
  for (const b of document.querySelectorAll('#menue button')) {
    if (b.dataset.seite === neu) b.setAttribute('aria-current', 'page');
    else b.removeAttribute('aria-current');
  }
  $('#kopf-titel').textContent = titel[neu];
  $('#kopf-pfad').textContent = neu === 'heute' && stand ? tagLang(stand.heute) : '';
  if (neu !== 'handy') { clearInterval(uhr); uhr = null; }
  zeichne();
}

/* ---------- Verdrahten ---------- */

document.addEventListener('click', (e) => {
  const b = e.target.closest('#menue button');
  if (b) zeigeSeite(b.dataset.seite);
});

$('#form-ziel').addEventListener('submit', (e) => {
  e.preventDefault();
  const kcal = Number(e.target.kcal.value);
  laden('/api/ziel', 'POST', { kcal });
});

$('#knopf-ausfuhr').addEventListener('click', async () => {
  try {
    const antwort = await hole('/api/ausfuhr', 'POST');
    melde('Geschrieben: ' + antwort.pfad, 9000);
  } catch (fehler) {
    melde(fehler.message);
  }
});

// Ein neuer Eintrag kommt nicht von hier, sondern vom Handy. Ohne
// Nachfragen bliebe das Fenster stehen, waehrend der Tag weiterlaeuft —
// deshalb alle zehn Sekunden neu holen. Das ist ein paar Kilobyte an einen
// Dienst auf demselben Rechner.
setInterval(() => { if (!uhr) laden(); }, 10000);

// Zurueck aus dem Hintergrund: sofort nachsehen, statt bis zu zehn Sekunden
// eine alte Zahl zu zeigen.
document.addEventListener('visibilitychange', () => { if (!document.hidden) laden(); });

laden();
