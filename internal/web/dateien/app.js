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
  if (seite === 'einkauf') einkauf();
  if (seite === 'handy') handy();
  if (seite === 'ki') ki();
  kopfSchub();
  schaetzTakt();
  planTakt();
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
  setzeZahl('#zahl-einkauf', (stand.einkauf && stand.einkauf.posten || []).filter((p) => !p.erledigt).length);
  setzeZahl('#zahl-handy', stand.geraete.length);
  kiZahl();
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

  offenKarte();

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

// quellen sagt, was an einer Zahl anders ist als nachgeschlagen. Aus der
// Datenbank steht nichts dabei — das ist der Normalfall.
const quellen = {
  hand: 'von Hand',
  ki: 'geschätzt',
  eigen: 'eigene DB',
  offen: 'ohne Werte',
};

function eintragszeile(z) {
  const offen = z.quelle === 'offen';
  const r = el('div', 'eintrag' + (offen ? ' offen' : ''));
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
  const unten = el('span');
  if (quellen[z.quelle]) unten.appendChild(el('em', 'quelle', quellen[z.quelle]));
  unten.appendChild(document.createTextNode(teile.join(' · ')));
  was.appendChild(unten);
  // Worauf eine Schaetzung beruht, steht im Titel: nachlesbar, ohne dass
  // jede Zeile einen Satz mitschleppt.
  const s = z.schaetzung;
  if (s) {
    r.title = [s.begruendung, s.sicherheit && 'Sicherheit: ' + s.sicherheit,
               s.nameVorher && 'am Handy: „' + s.nameVorher + '"'].filter(Boolean).join('\n');
  }
  r.appendChild(was);

  // Ein offener Eintrag zaehlt mit null. Eine 0 hinzuschreiben hiesse aber,
  // er haette keine Kalorien — deshalb das Fragezeichen.
  const k = el('div', 'kcal');
  k.appendChild(el('b', null, offen ? '?' : zahl(z.kcal)));
  k.appendChild(el('span', null, 'kcal'));
  r.appendChild(k);

  const werkzeuge = el('div', 'werkzeuge');
  if (z.quelle === 'hand' || z.quelle === 'ki' || z.quelle === 'eigen') {
    werkzeuge.appendChild(knopf('i-ki', 'Von Claude neu schätzen lassen', () => {
      laden('/api/eintrag/' + encodeURIComponent(z.id) + '/schaetzen', 'POST');
      melde('„' + z.name + '" wird neu geschätzt.');
    }));
  }
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

/* ---------- Ohne Naehrwerte ---------- */

// Eintraege, die am Handy nur Name und Menge bekamen, schaetzt Claude von
// allein, sobald sie hier ankommen. Die Karte zeigt, was noch fehlt und was
// gerade passiert — und ist weg, sobald nichts mehr fehlt.
let schaetzUhr = null;
let schaetzteGerade = false;

function offenKarte() {
  const karte = $('#offen-karte');
  const st = stand.schaetzung || {};
  const offen = stand.offen || [];

  if (!offen.length && !st.laeuft) {
    karte.hidden = true;
    karte.replaceChildren();
    return;
  }
  karte.hidden = false;

  const h = el('h2', null, 'Ohne Nährwerte');
  h.appendChild(el('span', 'zusatz',
    offen.length + (offen.length === 1 ? ' Eintrag zählt' : ' Einträge zählen') + ' noch mit null'));

  const lage = el('p', 'lage');
  if (st.laeuft) {
    lage.appendChild(el('span', 'schlag'));
    const wieviele = st.anzahl ? st.anzahl + (st.anzahl === 1 ? ' Eintrag' : ' Einträge') : 'Einträge';
    lage.appendChild(document.createTextNode('Claude (Haiku) schätzt gerade ' + wieviele + ' …'));
  } else if (!st.bereit) {
    lage.textContent = st.hinweis;
  } else if (st.fehler) {
    lage.classList.add('fehler');
    lage.textContent = 'Das Schätzen ist gescheitert: ' + st.fehler;
  } else {
    lage.textContent = 'Claude schätzt von allein, was vom Handy ohne Werte kommt. ' +
      'Was geschätzt ist, zählt mit und landet in der eigenen Datenbank.';
  }

  const liste = el('div');
  for (const z of offen) {
    const zeile = eintragszeile(z);
    // Das Datum gehoert hier dazu: offen kann auch etwas von gestern sein,
    // das erst jetzt vom Handy kam.
    zeile.querySelector('.uhr').textContent = z.tag === stand.heute ? z.uhr : tagKurz(z.tag);
    const grund = st.unklar && st.unklar[z.id];
    if (grund) {
      const span = zeile.querySelector('.was span');
      span.appendChild(document.createTextNode(' · ' + grund));
      zeile.title = grund;
    }
    liste.appendChild(zeile);
  }

  const schub = el('div', 'schub');
  if (st.laeuft) {
    const halt = el('button', 'knopf', 'Abbrechen');
    halt.addEventListener('click', () => laden('/api/schaetzen/abbruch', 'POST'));
    schub.appendChild(halt);
  } else if (offen.length) {
    const los = el('button', 'knopf haupt', 'Jetzt schätzen');
    los.disabled = !st.bereit;
    los.addEventListener('click', () => laden('/api/schaetzen', 'POST'));
    schub.appendChild(los);
  }

  karte.replaceChildren(h, lage, liste, schub);
}

// schaetzTakt fragt oefter nach, solange geschaetzt wird — sonst stuende die
// fertige Zahl bis zu zehn Sekunden lang nicht da. Und wenn es fertig ist,
// sagt es das auch auf den anderen Seiten.
function schaetzTakt() {
  const st = stand.schaetzung || {};
  if (st.laeuft && !schaetzUhr) schaetzUhr = setInterval(() => laden(), 2000);
  if (!st.laeuft && schaetzUhr) { clearInterval(schaetzUhr); schaetzUhr = null; }

  if (schaetzteGerade && !st.laeuft && !st.fehler) {
    const rest = (stand.offen || []).length;
    melde(rest ? 'Geschätzt — ' + rest + ' noch ohne Werte.' : 'Geschätzt — alles hat jetzt Nährwerte.');
  }
  schaetzteGerade = !!st.laeuft;
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
  eigene();
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

function eigene() {
  const liste = stand.eigene || [];
  $('#eigene-zusatz').textContent = liste.length
    ? liste.length + (liste.length === 1 ? ' Lebensmittel' : ' Lebensmittel') : '';
  const wohin = $('#eigene-liste');
  if (!liste.length) {
    wohin.replaceChildren(el('div', 'leer', 'Noch leer. Hier landet, was Claude geschätzt hat.'));
    return;
  }
  wohin.replaceChildren(...liste.map((e) => {
    const r = el('div', 'produkt');
    const was = el('div', 'was');
    was.appendChild(el('b', null, e.name));
    const teile = [];
    if (e.marke) teile.push(e.marke);
    if (e.sicherheit) teile.push('Sicherheit ' + e.sicherheit);
    if (e.begruendung) teile.push(e.begruendung);
    if (e.barcode) teile.push(e.barcode);
    was.appendChild(el('span', null, teile.join(' · ')));
    r.appendChild(was);

    const werte = el('div', 'werte');
    const n = e.je100;
    werte.appendChild(wertfeld(zahl(n.kcal), 'kcal/100 ' + (e.einheit || 'g')));
    werte.appendChild(wertfeld(gramm(n.fett), 'Fett'));
    werte.appendChild(wertfeld(gramm(n.kohlenhydrate), 'KH'));
    werte.appendChild(wertfeld(gramm(n.eiweiss), 'Eiweiß'));
    r.appendChild(werte);

    r.appendChild(knopf('i-weg', 'Aus der eigenen Datenbank nehmen', () => {
      laden('/api/eigene/' + encodeURIComponent(e.id), 'DELETE');
      melde('„' + e.name + '" aus der eigenen Datenbank genommen. Die Einträge behalten ihre Werte.');
    }));
    return r;
  }));
}

/* ---------- Einkauf ---------- */

/* Die Einkaufsliste ist der eine Teil, der zum Handy laeuft statt von ihm her:
 * geschrieben wird hier, abgehakt wird im Laden. Deshalb wird sie hier auch
 * nicht nur nach der eigenen Aenderung neu gezeichnet, sondern bei jedem
 * Nachfragen — waehrend man hinsieht, kann jemand mit dem Handy schon drei
 * Posten abgehakt haben.
 *
 * Preise stehen hier keine. Es gibt Mengen, und die verstellt man mit + und −;
 * wie gross ein Schritt ist, sagt die Einheit, und die kommt aus dem
 * Tagebuch — damit das Plus hier und das Plus am Handy dasselbe tun. */

let planUhr = null;
let planLief = false;

const planVorschlaege = [
  'Einkauf für die nächsten drei Tage',
  'Wochenendeinkauf',
  'Was ich sowieso ständig esse',
  'Mehr Eiweiß, weniger Zucker',
  'Nur Frühstück für die Woche',
];

function einkauf() {
  planKarte();
  einkaufListe();
  einkaufSchub();
  neuFelder();
}

// planKarte ist der Auftrag an Claude: ein Wunsch, ein Knopf, und waehrend es
// laeuft der Stand. Ohne claude-CLI steht hier, warum nichts geht — bevor
// jemand tippt und nicht danach.
function planKarte() {
  const pl = stand.plan || {};
  const e = stand.einkauf || {};

  $('#plan-zusatz').textContent = e.erstellt
    ? 'zuletzt ' + zeitpunkt(e.erstellt) + (e.modell ? ' · ' + e.modell : '') : '';

  const lage = $('#plan-lage');
  lage.className = 'lage';
  lage.replaceChildren();
  if (pl.laeuft) {
    lage.appendChild(el('span', 'schlag'));
    lage.appendChild(document.createTextNode(
      'Claude (Haiku) schreibt' + (pl.wunsch ? ' — „' + pl.wunsch + '“' : ' die Liste') + ' …'));
  } else if (!pl.bereit) {
    lage.textContent = pl.hinweis;
  } else if (pl.fehler) {
    lage.classList.add('fehler');
    lage.textContent = 'Das Schreiben ist gescheitert: ' + pl.fehler;
  }

  const gesperrt = !!pl.laeuft || !pl.bereit;
  $('#plan-text').disabled = gesperrt;
  $('#plan-senden').disabled = gesperrt;
  $('#plan-senden').textContent = pl.laeuft ? 'läuft …' : 'Schreiben';
  $('#plan-ergaenzen').disabled = gesperrt;

  const v = $('#plan-vorschlaege');
  if (!v.childElementCount) {
    v.replaceChildren(...planVorschlaege.map((t) => {
      const b = el('button', null, t);
      b.type = 'button';
      b.addEventListener('click', () => { $('#plan-text').value = t; planSenden(); });
      return b;
    }));
  }
  for (const b of v.children) b.disabled = gesperrt;

  // Der Abbruch steht in der Kopfzeile, wo auch der des KI-Reiters steht.
  kopfSchub();
}

async function planSenden() {
  const feld = $('#plan-text');
  const wunsch = feld.value.trim();
  const ergaenzen = $('#plan-ergaenzen').checked;
  const vorher = (stand.einkauf && stand.einkauf.posten || []).length;
  if (!ergaenzen && vorher) {
    melde(vorher + (vorher === 1 ? ' Posten wird' : ' Posten werden') + ' durch die neue Liste ersetzt.');
  }
  await laden('/api/einkauf/erstellen', 'POST', { wunsch, ergaenzen });
  if (stand.plan && stand.plan.laeuft) feld.value = '';
}

function einkaufListe() {
  const posten = (stand.einkauf && stand.einkauf.posten) || [];
  const offen = posten.filter((p) => !p.erledigt).length;
  const wohin = $('#einkauf-liste');

  $('#einkauf-zusatz').textContent = posten.length
    ? offen + ' von ' + posten.length + ' offen' : '';

  const notiz = $('#einkauf-notiz');
  const text = (stand.einkauf && stand.einkauf.notiz) || '';
  notiz.hidden = !text;
  notiz.textContent = text;

  // Nicht neu bauen, solange jemand in einer Zeile eine Menge eintippt: das
  // Fenster fragt alle zehn Sekunden nach, und das Feld waere weg, bevor die
  // Zahl fertig ist.
  if (wohin.contains(document.activeElement) && document.activeElement.tagName === 'INPUT') return;

  if (!posten.length) {
    wohin.replaceChildren(el('div', 'leer',
      'Noch keine Liste. Oben einen Wunsch eintippen — oder unten selbst einen Posten dazuschreiben.'));
    return;
  }

  // Nach Abteilung gebuendelt, in der Reihenfolge, in der man den Laden
  // durchgeht. Die kommt vom Dienst; hier wird nur gruppiert.
  const gruppen = [];
  for (const p of posten) {
    let g = gruppen[gruppen.length - 1];
    if (!g || g.name !== p.gruppe) {
      g = { name: p.gruppe, zeilen: [] };
      gruppen.push(g);
    }
    g.zeilen.push(p);
  }

  wohin.replaceChildren(...gruppen.map((g) => {
    const kasten = el('div', 'mahlzeit');
    const h = el('h3', null, g.name);
    const offenHier = g.zeilen.filter((p) => !p.erledigt).length;
    h.appendChild(el('span', 'summe', offenHier ? offenHier + ' offen' : 'erledigt'));
    kasten.appendChild(h);
    for (const p of g.zeilen) kasten.appendChild(postenZeile(p));
    return kasten;
  }));
}

function postenZeile(p) {
  const r = el('div', 'posten' + (p.erledigt ? ' erledigt' : ''));

  const haken = el('button', 'haken');
  haken.type = 'button';
  haken.setAttribute('aria-pressed', p.erledigt ? 'true' : 'false');
  haken.title = p.erledigt ? 'Wieder offen' : 'Im Wagen';
  haken.setAttribute('aria-label', haken.title);
  if (p.erledigt) haken.appendChild(bild('i-haken', 12));
  haken.addEventListener('click', () => {
    laden('/api/einkauf/posten/' + encodeURIComponent(p.id) + '/haken', 'POST', { erledigt: !p.erledigt });
  });
  r.appendChild(haken);

  const was = el('div', 'was');
  was.appendChild(el('b', null, p.name));
  // Die Marke steht nur an dem, was nicht von Claude kam: auf einer Liste, die
  // er geschrieben hat, waere sie an jeder Zeile und sagte damit nichts mehr.
  const unten = el('span');
  if (p.von === 'hand') unten.appendChild(el('em', 'quelle', 'selbst'));
  unten.appendChild(document.createTextNode(p.grund || (p.von === 'hand' ? 'dazugeschrieben' : p.gruppe)));
  was.appendChild(unten);
  was.addEventListener('click', () => {
    laden('/api/einkauf/posten/' + encodeURIComponent(p.id) + '/haken', 'POST', { erledigt: !p.erledigt });
  });
  r.appendChild(was);

  r.appendChild(mengeFeld(p));

  const werkzeuge = el('div', 'werkzeuge');
  werkzeuge.appendChild(knopf('i-weg', 'Von der Liste nehmen', () => {
    laden('/api/einkauf/posten/' + encodeURIComponent(p.id), 'DELETE');
    melde('„' + p.name + '“ von der Liste genommen.');
  }));
  r.appendChild(werkzeuge);
  return r;
}

// mengeFeld ist − Zahl +. Die Zahl selbst ist ein Knopf: ein Druck darauf
// macht ein Feld daraus, in dem Menge und Einheit auch getauscht werden
// koennen. Ueber + und − geht es sonst in Schritten, die zur Einheit passen —
// 50 g beim Mehl, ein Stueck beim Ei.
function mengeFeld(p) {
  const kasten = el('div', 'menge');

  const weniger = el('button', 'stufe');
  weniger.type = 'button';
  weniger.title = 'weniger';
  weniger.setAttribute('aria-label', 'weniger');
  weniger.appendChild(bild('i-minus', 13));
  weniger.addEventListener('click', () => schritt(p, -1));

  const wert = el('button', 'wert', mengeText(p));
  wert.type = 'button';
  wert.title = 'Menge und Einheit ändern';
  wert.addEventListener('click', () => mengeTauschen(kasten, p));

  const mehr = el('button', 'stufe');
  mehr.type = 'button';
  mehr.title = 'mehr';
  mehr.setAttribute('aria-label', 'mehr');
  mehr.appendChild(bild('i-plus', 13));
  mehr.addEventListener('click', () => schritt(p, 1));

  kasten.replaceChildren(weniger, wert, mehr);
  return kasten;
}

function schritt(p, richtung) {
  laden('/api/einkauf/posten/' + encodeURIComponent(p.id) + '/schritt', 'POST', { richtung });
}

// mengeTauschen macht aus der Zahl ein Feld — Menge tippen, Einheit wählen.
// Enter und der Wechsel der Einheit schicken ab, Esc lässt alles, wie es war.
function mengeTauschen(kasten, p) {
  const zahl = el('input');
  zahl.type = 'number';
  zahl.min = '0';
  zahl.step = '0.5';
  zahl.value = String(p.menge);
  zahl.inputMode = 'decimal';

  const einheit = einheitenWahl(p.einheit);

  const fertig = () => {
    const menge = Number(zahl.value.replace(',', '.'));
    if (!(menge > 0)) { einkaufListe(); return; }
    laden('/api/einkauf/posten/' + encodeURIComponent(p.id) + '/menge', 'POST',
          { menge, einheit: einheit.value });
  };
  zahl.addEventListener('keydown', (ev) => {
    if (ev.key === 'Enter') { ev.preventDefault(); fertig(); }
    if (ev.key === 'Escape') { ev.preventDefault(); einkaufListe(); }
  });
  einheit.addEventListener('change', fertig);
  zahl.addEventListener('blur', () => setTimeout(() => {
    if (!kasten.contains(document.activeElement)) fertig();
  }, 0));

  kasten.replaceChildren(zahl, einheit);
  zahl.focus();
  zahl.select();
}

// einheitenWahl baut die Auswahl aus dem, was der Dienst mitgeschickt hat —
// die Oberflaeche kennt die Einheiten nicht selbst.
function einheitenWahl(gewaehlt) {
  const s = el('select');
  for (const e of stand.einheiten || []) {
    const o = el('option', null, e.viele === e.eins ? e.eins : e.eins + ' / ' + e.viele);
    o.value = e.name;
    if (e.name === gewaehlt) o.selected = true;
    s.appendChild(o);
  }
  return s;
}

function einheitVon(name) {
  return (stand.einheiten || []).find((e) => e.name === name) || { eins: name, viele: name };
}

function mengeText(p) {
  const e = einheitVon(p.einheit);
  const menge = Number(p.menge || 0);
  const name = Math.abs(menge - 1) < 0.001 ? e.eins : e.viele;
  return menge.toLocaleString('de-DE', { maximumFractionDigits: 3 }) + ' ' + name;
}

function einkaufSchub() {
  const posten = (stand.einkauf && stand.einkauf.posten) || [];
  const fertig = posten.filter((p) => p.erledigt).length;
  const schub = $('#einkauf-schub');
  const teile = [];

  if (fertig) {
    const weg = el('button', 'knopf', 'Erledigte wegräumen (' + fertig + ')');
    weg.type = 'button';
    weg.addEventListener('click', () => {
      laden('/api/einkauf/aufraeumen', 'POST');
      melde(fertig + (fertig === 1 ? ' Posten weggeräumt.' : ' Posten weggeräumt.'));
    });
    teile.push(weg);
  }
  if (posten.length) {
    const leer = el('button', 'knopf gefahr', 'Liste leeren');
    leer.type = 'button';
    leer.addEventListener('click', () => {
      laden('/api/einkauf/leeren', 'POST');
      melde('Liste geleert.');
    });
    teile.push(leer);

    const hinweis = el('span', 'randnotiz',
      'Geht beim nächsten Abgleich aufs Handy — dort wird abgehakt.');
    teile.push(hinweis);
  }
  schub.replaceChildren(...teile);
}

// neuFelder fuellt die beiden Auswahlfelder unter der Liste. Einmal: sie
// aendern sich nicht, und ein Neubau wuerde die Wahl jedes Mal zurueckwerfen.
function neuFelder() {
  const einheit = $('#neu-einheit');
  if (!einheit.childElementCount) {
    for (const e of stand.einheiten || []) {
      const o = el('option', null, e.viele === e.eins ? e.eins : e.eins + ' / ' + e.viele);
      o.value = e.name;
      einheit.appendChild(o);
    }
  }
  const gruppe = $('#neu-gruppe');
  if (!gruppe.childElementCount) {
    for (const g of stand.gruppen || []) {
      const o = el('option', null, g);
      o.value = g;
      if (g === 'Sonstiges') o.selected = true;
      gruppe.appendChild(o);
    }
  }
}

// planTakt fragt oefter nach, solange geschrieben wird — und sagt Bescheid,
// wenn die Liste da ist, auch wenn gerade eine andere Seite offen steht.
function planTakt() {
  const pl = stand.plan || {};
  if (pl.laeuft && !planUhr) planUhr = setInterval(() => laden(), 1500);
  if (!pl.laeuft && planUhr) { clearInterval(planUhr); planUhr = null; }

  if (planLief && !pl.laeuft) {
    const offen = ((stand.einkauf && stand.einkauf.posten) || []).filter((p) => !p.erledigt).length;
    melde(pl.fehler ? 'Die Liste ist gescheitert: ' + pl.fehler
                    : 'Liste steht — ' + offen + (offen === 1 ? ' Posten' : ' Posten') + ' zu holen.');
  }
  planLief = !!pl.laeuft;
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

/* ---------- KI ---------- */

/* Der KI-Reiter haengt nicht am Tagebuch-Stand, sondern hat einen eigenen:
 * er aendert sich in ganz anderem Takt. Waehrend eine Antwort entsteht, wird
 * im Sekundentakt nachgefragt, und dabei jedes Mal Jahresraster und
 * Produktliste mitzubauen waere Arbeit ohne Ertrag.
 *
 * Hier wird ausserdem als Einziges Markdown gezeichnet — aber weiter mit
 * createElement. Der Text kommt zwar von Claude und nicht von einem Handy,
 * doch er enthaelt Produktnamen aus einer fremden Datenbank, und die sind
 * genau dasselbe Problem wie nebenan.
 */
let kiStand = null;
let kiUhr = null;       // laeuft nur, solange eine Antwort entsteht
let kiMarke = '';       // woran erkannt wird, dass sich am Verlauf nichts tat
let kiStromMarke = '';
let kopfMarke = '';

const vorschlaege = [
  'Wie war mein Tag?',
  'Was fehlt mir heute noch?',
  'Woher kommen heute die Kalorien?',
  'Vergleich mit der letzten Woche',
  'Wo steckt der meiste Zucker?',
  'Komme ich auf genug Eiweiß?',
];

async function kiHolen(pfad = '/api/ki', art = 'GET', rumpf) {
  try {
    kiStand = await hole(pfad, art, rumpf);
  } catch (fehler) {
    melde(fehler.message);
    return;
  }
  kiTakt();
  kiZahl();
  kopfSchub();
  if (seite === 'ki') ki();
}

// Die Uhr laeuft nur, solange eine Antwort entsteht — dann aber auch auf den
// anderen Seiten weiter: der Punkt im Menue soll Bescheid sagen, wenn sie da
// ist, ohne dass jemand danach sieht.
function kiTakt() {
  const laeuft = !!(kiStand && kiStand.laeuft);
  if (laeuft && !kiUhr) kiUhr = setInterval(() => kiHolen(), 900);
  if (!laeuft && kiUhr) { clearInterval(kiUhr); kiUhr = null; }
}

function kiZahl() {
  const k = $('#zahl-ki');
  const laeuft = !!(kiStand && kiStand.laeuft);
  k.textContent = laeuft ? '·' : '';
  k.classList.toggle('leer', !laeuft);
  k.classList.toggle('laeuft', laeuft);
}

function ki() {
  if (!kiStand) {
    $('#ki-verlauf').replaceChildren(el('div', 'leer', 'Einen Moment …'));
    return;
  }
  kiVerlauf();
  kiStrom();
  kiEingabe();
}

function kiVerlauf() {
  const nachrichten = (kiStand.gespraech && kiStand.gespraech.nachrichten) || [];
  const wohin = $('#ki-verlauf');

  // Ohne Marke waere der Verlauf im Sekundentakt neu gebaut: aufgeklappte
  // Mitschnitte klappten zu und eine markierte Zeile waere jedes Mal weg.
  const letzte = nachrichten[nachrichten.length - 1];
  const marke = [nachrichten.length, letzte ? letzte.text.length : 0,
                 kiStand.laeuft, kiStand.bereit].join('|');
  if (marke === kiMarke) return;

  // Nur nachrollen, wer ohnehin unten stand. Wer weiter oben liest, soll nicht
  // von der nächsten Antwort weggerissen werden.
  const unten = wohin.scrollHeight - wohin.scrollTop - wohin.clientHeight < 90;
  kiMarke = marke;

  if (!nachrichten.length) {
    wohin.replaceChildren(kiLeer());
    return;
  }
  const liste = el('div', 'nachrichten');
  for (const n of nachrichten) liste.appendChild(nachrichtZeile(n));
  wohin.replaceChildren(liste);
  if (unten) wohin.scrollTop = wohin.scrollHeight;
}

function kiLeer() {
  const k = el('div', 'kileer');
  k.appendChild(el('h2', null, 'Frag nach dem Tag'));
  k.appendChild(el('p', null,
    'Vor jeder Frage geht der Stand des Tagebuchs mit: der heutige Tag mit jedem ' +
    'Eintrag, die Tage davor als Zeilen. Für alles Ältere liest Claude in ' +
    'tagebuch.json nach. Ändern kann es nichts — ein Eintrag bleibt, was er ist.'));
  k.appendChild(el('p', 'klein',
    'Gerufen wird die angemeldete claude-CLI auf diesem Rechner, nicht die API. ' +
    'Nach draußen geht dabei die Frage samt Stand — sonst nichts.'));

  if (!kiStand.bereit) {
    const warnung = el('p', 'klein', kiStand.hinweis);
    warnung.style.color = 'var(--tinte)';
    k.appendChild(warnung);
    return k;
  }

  const v = el('div', 'vorschlaege');
  for (const t of vorschlaege) {
    const b = el('button', null, t);
    b.type = 'button';
    b.addEventListener('click', () => { $('#ki-text').value = t; kiSenden(); });
    v.appendChild(b);
  }
  k.appendChild(v);
  return k;
}

function nachrichtZeile(n) {
  if (n.rolle === 'trenner') return el('div', 'trenner', n.text || 'Neue Unterhaltung');

  const k = el('div', 'nachricht ' + (n.rolle === 'du' ? 'du' : 'claude') +
                                     (n.fehler ? ' fehlgeschlagen' : ''));

  const wer = el('div', 'wer', n.rolle === 'du' ? 'Du' : 'Claude');
  const zusatz = [];
  if (n.zeit) zusatz.push(zeitpunkt(n.zeit));
  if (n.dauer) zusatz.push(n.dauer);
  if (zusatz.length) wer.appendChild(el('span', 'wann', zusatz.join(' · ')));
  k.appendChild(wer);

  const inhalt = el('div', 'inhalt');
  // Die eigene Frage und eine Fehlermeldung stehen wörtlich da. Markdown
  // gezeichnet wird nur, was Claude geschrieben hat.
  if (n.rolle === 'du' || n.fehler) inhalt.textContent = n.text;
  else inhalt.appendChild(markdown(n.text));
  k.appendChild(inhalt);

  if (n.schritte && n.schritte.length) k.appendChild(mitschnitt(n.schritte));
  return k;
}

// mitschnitt haelt nach, was Claude unterwegs getan hat — zugeklappt, damit es
// nachlesbar ist, ohne im Weg zu stehen.
function mitschnitt(schritte) {
  const d = el('details', 'mitschnitt');
  d.appendChild(el('summary', null,
    schritte.length + (schritte.length === 1 ? ' Schritt' : ' Schritte')));
  const strom = el('div', 'strom');
  for (const s of schritte) strom.appendChild(schrittZeile(s));
  d.appendChild(strom);
  return d;
}

function schrittZeile(s) {
  if (s.art === 'text') return el('div', 'schritt text', s.titel);
  const z = el('div', 'schritt ' + (s.art || 'hinweis'));
  z.appendChild(el('span', 'titel', s.titel));
  z.appendChild(el('span', 'detail', s.detail || (s.offen ? '…' : '')));
  return z;
}

function kiStrom() {
  const kasten = $('#ki-strom');
  const lauf = kiStand.lauf;
  if (!lauf) {
    kasten.hidden = true;
    kasten.replaceChildren();
    kiStromMarke = '';
    return;
  }
  kasten.hidden = false;

  const schritte = lauf.schritte || [];
  const jetzt = schritte[schritte.length - 1];
  const sekunden = Math.max(0, Math.round((Date.now() - new Date(lauf.beginn)) / 1000));
  const marke = [schritte.length, jetzt ? jetzt.titel.length : 0,
                 jetzt ? jetzt.detail : '', lauf.modell, sekunden].join('|');
  if (marke === kiStromMarke) return;
  const unten = kasten.scrollHeight - kasten.scrollTop - kasten.clientHeight < 70;
  kiStromMarke = marke;

  const kopf = el('div', 'kopfzeile');
  kopf.appendChild(el('span', 'puls'));
  kopf.appendChild(el('span', null, lauf.modell || 'claude'));
  const rechts = [dauer(sekunden)];
  if (lauf.tokenAus) rechts.push(zahl(lauf.tokenAus) + ' Token');
  kopf.appendChild(el('span', 'rechts', rechts.join(' · ')));

  const teile = [kopf];
  if (!schritte.length) teile.push(el('div', 'schritt', 'Claude liest den Tag …'));
  for (const s of schritte) teile.push(schrittZeile(s));
  kasten.replaceChildren(...teile);
  if (unten) kasten.scrollTop = kasten.scrollHeight;
}

function kiEingabe() {
  const gesperrt = !!kiStand.laeuft || !kiStand.bereit;
  $('#ki-text').disabled = gesperrt;
  $('#ki-senden').disabled = gesperrt;
  $('#ki-senden').textContent = kiStand.laeuft ? 'läuft …' : 'Senden';

  const h = $('#ki-hinweis');
  if (!kiStand.bereit) h.textContent = kiStand.hinweis;
  else if (kiStand.laeuft) h.textContent = 'Claude denkt nach. Abbrechen steht oben rechts.';
  else h.textContent = 'Enter schickt ab, Umschalt+Enter macht eine neue Zeile.';
}

async function kiSenden() {
  const feld = $('#ki-text');
  const text = feld.value.trim();
  if (!text) return;
  if (kiStand && !kiStand.bereit) { melde(kiStand.hinweis); return; }

  feld.value = '';
  hoeheAnpassen(feld);
  try {
    kiStand = await hole('/api/ki/frage', 'POST', { text });
  } catch (fehler) {
    // Die Frage hat jemand getippt — sie darf nicht mit dem Fehler verschwinden.
    feld.value = text;
    hoeheAnpassen(feld);
    melde(fehler.message);
    return;
  }
  kiTakt();
  kiZahl();
  kopfSchub();
  ki();
  feld.focus();
}

// hoeheAnpassen laesst das Feld mit dem Text wachsen. Eine feste Zeile waere
// bei einer Frage ueber drei Zeilen ein Guckloch.
function hoeheAnpassen(feld) {
  feld.style.height = 'auto';
  feld.style.height = feld.scrollHeight + 'px';
}

function dauer(sekunden) {
  if (sekunden < 60) return sekunden + ' s';
  return Math.floor(sekunden / 60) + ':' + String(sekunden % 60).padStart(2, '0') + ' min';
}

function zeitpunkt(rfc) {
  const d = new Date(rfc);
  if (isNaN(d)) return '';
  const uhrzeit = d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' });
  if (d.toDateString() === new Date().toDateString()) return uhrzeit;
  return d.getDate() + '.' + (d.getMonth() + 1) + '. ' + uhrzeit;
}

// kopfSchub fuellt die rechte Seite der Kopfzeile mit dem, was auf dieser
// Seite zu tun ist. Bisher war sie leer — und der KI-Reiter ist der erste mit
// Knoepfen, die nicht in eine Karte gehoeren.
function kopfSchub() {
  const schub = $('#kopf-schub');
  const nachrichten = (kiStand && kiStand.gespraech && kiStand.gespraech.nachrichten) || [];
  const plan = (stand && stand.plan) || {};
  const marke = [seite, !!(kiStand && kiStand.laeuft), nachrichten.length > 0, !!plan.laeuft].join('|');
  if (marke === kopfMarke) return;
  kopfMarke = marke;

  // Der Einkauf hat genau einen Knopf hier, und nur, solange etwas laeuft:
  // die Notbremse. Alles andere steht in seinen Karten.
  if (seite === 'einkauf') {
    schub.replaceChildren(...(plan.laeuft
      ? [kopfknopf('i-stopp', 'Abbrechen', () => laden('/api/einkauf/abbruch', 'POST'))]
      : []));
    return;
  }

  if (seite !== 'ki' || !kiStand) { schub.replaceChildren(); return; }

  const teile = [];
  if (kiStand.laeuft) {
    teile.push(kopfknopf('i-stopp', 'Abbrechen', () => kiHolen('/api/ki/abbruch', 'POST')));
  } else if (nachrichten.length) {
    teile.push(kopfknopf('i-neu', 'Neue Unterhaltung', () => {
      kiHolen('/api/ki/neu', 'POST');
      melde('Claude fängt bei der nächsten Frage von vorn an. Was dasteht, bleibt stehen.');
    }));
    teile.push(knopf('i-weg', 'Verlauf löschen', () => {
      kiHolen('/api/ki/vergessen', 'POST');
      melde('Verlauf gelöscht. Die Einträge im Tagebuch sind davon nicht betroffen.');
    }));
  }
  schub.replaceChildren(...teile);
}

function kopfknopf(symbol, text, tat) {
  const b = el('button', 'knopf');
  b.type = 'button';
  b.appendChild(bild(symbol, 13));
  b.appendChild(document.createTextNode(text));
  b.addEventListener('click', tat);
  return b;
}

/* ---------- Markdown ---------- */

/* Nur so viel, wie in einer Antwort vorkommt: Ueberschriften, Listen,
 * Tabellen, Code, fett und kursiv. Gebaut wird mit createElement — eine
 * Zeichenkette an innerHTML zu geben waere hier der eine Ort, an dem der
 * Grundsatz von oben kippt. */

function markdown(text) {
  const frag = document.createDocumentFragment();
  const zeilen = String(text || '').replace(/\r\n?/g, '\n').split('\n');
  let i = 0;

  while (i < zeilen.length) {
    const z = zeilen[i];

    if (!z.trim()) { i++; continue; }

    // Codeblock: alles darin ist wörtlich, auch die Zeichen, die sonst etwas
    // bedeuten.
    if (/^\s*```/.test(z)) {
      const inhalt = [];
      i++;
      while (i < zeilen.length && !/^\s*```/.test(zeilen[i])) inhalt.push(zeilen[i++]);
      i++;
      const pre = el('pre');
      pre.appendChild(el('code', null, inhalt.join('\n')));
      frag.appendChild(pre);
      continue;
    }

    const kopf = z.match(/^\s*(#{1,6})\s+(.*)$/);
    if (kopf) {
      // Aus h1 und h2 wird h3: die Überschrift der Seite steht schon oben,
      // und eine zweite in derselben Größe wäre eine zweite Seite.
      const h = el('h' + Math.min(6, Math.max(3, kopf[1].length + 1)));
      h.appendChild(inline(kopf[2].replace(/\s*#+\s*$/, '')));
      frag.appendChild(h);
      i++;
      continue;
    }

    if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(z)) { frag.appendChild(el('hr')); i++; continue; }

    // Tabelle: Kopfzeile, dann eine Zeile aus Strichen. Ohne die zweite Zeile
    // ist ein Senkrechtstrich nur ein Senkrechtstrich.
    if (z.includes('|') && i + 1 < zeilen.length && trennzeile(zeilen[i + 1])) {
      const kopfzeile = spalten(z);
      const form = ausrichtung(zeilen[i + 1]);
      i += 2;
      const koerper = [];
      while (i < zeilen.length && zeilen[i].includes('|') && zeilen[i].trim()) koerper.push(spalten(zeilen[i++]));
      frag.appendChild(tabelle(kopfzeile, form, koerper));
      continue;
    }

    if (/^\s*>\s?/.test(z)) {
      const inhalt = [];
      while (i < zeilen.length && /^\s*>\s?/.test(zeilen[i])) inhalt.push(zeilen[i++].replace(/^\s*>\s?/, ''));
      const q = el('blockquote');
      q.appendChild(markdown(inhalt.join('\n')));
      frag.appendChild(q);
      continue;
    }

    if (punktRegel(z)) {
      const geordnet = /^\s*\d+[.)]\s/.test(z);
      const liste = el(geordnet ? 'ol' : 'ul');
      while (i < zeilen.length) {
        const zeile = zeilen[i];
        if (!punktRegel(zeile) || /^\s*\d+[.)]\s/.test(zeile) !== geordnet) break;
        const teile = [zeile.replace(punktMuster, '')];
        i++;
        // Fortsetzungszeilen gehören zum selben Punkt, solange sie eingerückt
        // sind und keinen neuen beginnen.
        while (i < zeilen.length && zeilen[i].trim() && !punktRegel(zeilen[i]) &&
               /^\s{2,}/.test(zeilen[i]) && !/^\s*#{1,6}\s/.test(zeilen[i])) {
          teile.push(zeilen[i++].trim());
        }
        const li = el('li');
        li.appendChild(inline(teile.join(' ')));
        liste.appendChild(li);
      }
      frag.appendChild(liste);
      continue;
    }

    // Absatz: alles bis zur nächsten Leerzeile oder zum nächsten Block.
    const absatz = [];
    while (i < zeilen.length && zeilen[i].trim() && !blockBeginn(zeilen[i])) absatz.push(zeilen[i++].trim());
    if (!absatz.length) { i++; continue; }
    const p = el('p');
    p.appendChild(inline(absatz.join(' ')));
    frag.appendChild(p);
  }
  return frag;
}

const punktMuster = /^\s*(?:[-*+]|\d+[.)])\s+/;
const punktRegel = (z) => punktMuster.test(z);

function blockBeginn(z) {
  return punktRegel(z) || /^\s*(#{1,6}\s|>|```)/.test(z) || /^\s*([-*_])(\s*\1){2,}\s*$/.test(z);
}

function trennzeile(z) {
  return z.includes('-') && /^[\s|:-]+$/.test(z) && z.includes('|');
}

function spalten(z) {
  let s = z.trim();
  if (s.startsWith('|')) s = s.slice(1);
  if (s.endsWith('|')) s = s.slice(0, -1);
  return s.split('|').map((t) => t.trim());
}

// ausrichtung liest aus der Trennzeile, welche Spalte rechtsbündig steht. Das
// ist fast immer die mit den Zahlen — und die sollen untereinander stehen.
function ausrichtung(z) {
  return spalten(z).map((t) => t.endsWith(':') && !t.startsWith(':'));
}

function tabelle(kopfzeile, form, koerper) {
  const t = el('table');
  const kopf = el('thead');
  const kz = el('tr');
  kopfzeile.forEach((text, i) => {
    const th = el('th', form[i] ? 'zahl' : null);
    th.appendChild(inline(text));
    kz.appendChild(th);
  });
  kopf.appendChild(kz);
  t.appendChild(kopf);

  const rumpf = el('tbody');
  for (const zeile of koerper) {
    const tr = el('tr');
    kopfzeile.forEach((_, i) => {
      const td = el('td', form[i] ? 'zahl' : null);
      td.appendChild(inline(zeile[i] === undefined ? '' : zeile[i]));
      tr.appendChild(td);
    });
    rumpf.appendChild(tr);
  }
  t.appendChild(rumpf);
  return t;
}

// inline zeichnet aus, was in einer Zeile steht. Der Code kommt zuerst: was
// in Rückstrichen steht, ist wörtlich und wird nicht weiter ausgezeichnet.
function inline(text) {
  const frag = document.createDocumentFragment();
  for (const stueck of String(text).split(/(`[^`]+`)/g)) {
    if (!stueck) continue;
    if (stueck.length > 2 && stueck.startsWith('`') && stueck.endsWith('`')) {
      frag.appendChild(el('code', null, stueck.slice(1, -1)));
      continue;
    }
    fett(stueck, frag);
  }
  return frag;
}

function fett(text, wohin) {
  for (const stueck of text.split(/(\*\*[^*]+\*\*|__[^_]+__)/g)) {
    if (!stueck) continue;
    if (/^(\*\*|__)[\s\S]+\1$/.test(stueck)) {
      const b = el('strong');
      kursiv(stueck.slice(2, -2), b);
      wohin.appendChild(b);
      continue;
    }
    kursiv(stueck, wohin);
  }
}

// Kursiv nur mit Sternchen: Unterstriche stehen in Dateinamen und in
// Mahlzeitkürzeln, und die sollen nicht schräg werden.
function kursiv(text, wohin) {
  for (const stueck of text.split(/(\*[^*\n]+\*)/g)) {
    if (!stueck) continue;
    if (stueck.length > 2 && stueck.startsWith('*') && stueck.endsWith('*')) {
      wohin.appendChild(el('em', null, stueck.slice(1, -1)));
      continue;
    }
    wohin.appendChild(document.createTextNode(stueck));
  }
}

/* ---------- Seiten ---------- */

const titel = {
  heute: 'Heute', verlauf: 'Verlauf', produkte: 'Produkte',
  einkauf: 'Einkauf', handy: 'Handy', ki: 'KI',
};

function zeigeSeite(neu) {
  seite = neu;
  for (const s of document.querySelectorAll('.seite')) s.classList.toggle('an', s.id === 'seite-' + neu);
  for (const b of document.querySelectorAll('#menue button')) {
    if (b.dataset.seite === neu) b.setAttribute('aria-current', 'page');
    else b.removeAttribute('aria-current');
  }
  $('#kopf-titel').textContent = titel[neu];
  $('#kopf-pfad').textContent =
    neu === 'heute' && stand ? tagLang(stand.heute) :
    neu === 'einkauf' ? 'Hier geschrieben, im Laden abgehakt' :
    neu === 'ki' ? 'Claude, mit dem Tagebuch im Rücken' : '';
  if (neu !== 'handy') { clearInterval(uhr); uhr = null; }

  // Der KI-Reiter rollt selbst; die Seite darunter darf es dann nicht mehr.
  $('.blatt').classList.toggle('gespraechsseite', neu === 'ki');
  if (neu === 'ki') {
    if (!kiStand) kiHolen(); else ki();
    $('#ki-text').focus();
  }
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

$('#form-plan').addEventListener('submit', (e) => { e.preventDefault(); planSenden(); });
$('#plan-text').addEventListener('input', (e) => hoeheAnpassen(e.target));
$('#plan-text').addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); planSenden(); }
});

$('#form-posten').addEventListener('submit', async (e) => {
  e.preventDefault();
  const form = e.target;
  const name = form.name.value.trim();
  if (!name) { form.name.focus(); return; }
  const menge = Number(String(form.menge.value).replace(',', '.'));
  await laden('/api/einkauf/posten', 'POST', {
    name, menge: menge > 0 ? menge : 0,
    einheit: form.einheit.value, gruppe: form.gruppe.value,
  });
  form.name.value = '';
  form.menge.value = '';
  form.name.focus();
});

$('#form-ki').addEventListener('submit', (e) => { e.preventDefault(); kiSenden(); });
$('#ki-text').addEventListener('input', (e) => hoeheAnpassen(e.target));
// Enter schickt ab, Umschalt+Enter macht eine Zeile — wie in jedem Eingabefeld,
// in das man Saetze tippt. isComposing haelt die Zwischenablage der
// Tastaturbelegung heraus: waehrend dort ein Zeichen entsteht, ist Enter deren
// Bestaetigung und nicht unsere.
$('#ki-text').addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); kiSenden(); }
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
// Einmal auch den KI-Stand: laeuft dort noch eine Antwort aus der letzten
// Sitzung, soll der Punkt im Menue das sagen, bevor jemand hinsieht.
kiHolen();
