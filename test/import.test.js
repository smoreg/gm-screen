// Регресс импорта персонажей на вкладке «Партия».
// Гарантирует главное свойство: заливка НЕ ломает уже загруженную партию.
// Запуск: npm test  (node test/import.test.js). Ненулевой код выхода = провал.
const fs = require('fs');
const path = require('path');
const { JSDOM } = require('jsdom');

const htmlPath = path.join(__dirname, '..', 'web', 'index.html');
const html = fs.readFileSync(htmlPath, 'utf8');

const dom = new JSDOM(html, {
  runScripts: 'dangerously',
  pretendToBeVisual: true,
  url: 'http://localhost/', // включает window.localStorage
});
const { window } = dom;
// Сервера нет: fetch всегда падает — фолбэк не должен портить партию.
window.fetch = () => Promise.reject(new Error('no server'));

const party = () => { try { return JSON.parse(window.localStorage.getItem('gm-party-v1') || '[]'); } catch (e) { return null; } };
const len = () => { const p = party(); return p ? p.length : 'PARSE_ERR'; };
const names = () => { const p = party(); return p ? p.map(c => c.name) : ['PARSE_ERR']; };
const msg = () => window.document.getElementById('pcMsg').textContent;
function imp(text) {
  window.document.getElementById('pcText').value = text;
  window.document.getElementById('pcImport').dispatchEvent(new window.MouseEvent('click', { bubbles: true }));
}

let fails = 0;
const check = (name, cond) => { console.log((cond ? '  ok   ' : '  FAIL ') + name); if (!cond) fails++; };

setTimeout(() => {
  const okA = '{"schema":"gm-character/v1","name":"Тень","abilities":{"str":8,"dex":16,"con":14,"int":12,"wis":10,"cha":8}}';
  const okB = '{"schema":"gm-character/v1","name":"Мерген","abilities":{"str":12,"dex":16,"con":14,"int":10,"wis":15,"cha":8}}';

  check('старт: партия пуста', len() === 0);

  imp(okA);
  check('валидный JSON добавил 1 (Тень)', len() === 1 && names().includes('Тень'));

  imp('[' + okA + ',' + okB + ']');
  check('массив добавил ещё 2 (итого 3)', len() === 3);

  const before = len();
  imp('{это не json');
  check('битый JSON не тронул партию', len() === before);

  imp('{"schema":"gm-character/v1","name":"Безрукий"}'); // нет abilities
  check('JSON без abilities не тронул партию', len() === before);
  check('  сообщение «Партия не изменена»', /не изменена/i.test(msg()));

  imp('{"foo":123}'); // объект без name/abilities
  check('чужой объект не тронул партию', len() === before);

  check('целостность имён: Тень, Тень, Мерген',
    JSON.stringify(names()) === JSON.stringify(['Тень', 'Тень', 'Мерген']));

  console.log(fails ? ('\nПРОВАЛОВ: ' + fails) : '\nВСЕ ПРОВЕРКИ ЗЕЛЁНЫЕ');
  process.exit(fails ? 1 : 0);
}, 500);

// подстраховка от зависания
setTimeout(() => { console.error('TIMEOUT: инлайновые скрипты не отработали'); process.exit(2); }, 15000).unref?.();
