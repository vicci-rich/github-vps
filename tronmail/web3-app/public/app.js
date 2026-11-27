const $ = selector => document.querySelector(selector);

async function loadStatus() {
  const network = await fetch('/api/config').then(r => r.json());
  const status = await fetch('/api/status').then(r => r.json());
  $('#network').textContent = status.online
    ? `Shasta online — block ${status.block}`
    : `Shasta unavailable — ${status.error || 'RPC error'}`;
  if (!network.apiKeyConfigured) {
    $('#network').textContent += ' · no API key configured';
  }
}

$('#copy-genesis').addEventListener('click', async () => {
  await navigator.clipboard.writeText('python3 tronmail/deploy.py');
  $('#copy-genesis').textContent = 'Copied';
});

$('#build').addEventListener('click', async () => {
  $('#route-result').textContent = 'Building...';
  try {
    const response = await fetch('/api/build-route', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        source: $('#source').value.trim(),
        destination: $('#destination').value.trim(),
        token: $('#token').value.trim(),
        amount: $('#amount').value.trim(),
      }),
    });

    let mockLogId = null;
    let mergeId = null;

    $('#inject-log').addEventListener('click', async () => {
      $('#merge-result').textContent = 'Injecting mock log...';
      const response = await fetch('/api/mock-log', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          shastaEoa: $('#mock-shasta-eoa').value.trim(),
          amount: $('#mock-amount').value.trim(),
          mainnetAccounts: $('#mock-mainnet-eoas').value
            .split(/\s+/)
            .map(value => value.trim())
            .filter(Boolean),
        }),
      });
      const result = await response.json();
      mockLogId = result.log?.id || null;
      mergeId = result.merge?.id || null;
      $('#merge-result').textContent = JSON.stringify(result, null, 2);
    });

    $('#merge-eoas').addEventListener('click', async () => {
      if (!mockLogId) {
        $('#merge-result').textContent = 'Inject a mock log first.';
        return;
      }
      const mainnetAccounts = $('#mock-mainnet-eoas').value
        .split(/\s+/)
        .map(value => value.trim())
        .filter(Boolean);
      const response = await fetch('/api/merge', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mockLogId, mainnetAccounts }),
      });
      const result = await response.json();
      $('#merge-result').textContent = JSON.stringify(result, null, 2);
    });

    $('#publish-trongrid').addEventListener('click', async () => {
      if (!mergeId) {
        $('#merge-result').textContent = 'Inject a mock log and merge first.';
        return;
      }
      $('#merge-result').textContent = 'Building unsigned transaction at TronGrid...';
      const response = await fetch('/api/publish-mock-merge', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mergeId }),
      });
      const result = await response.json();
      $('#merge-result').textContent = JSON.stringify(result, null, 2);
    });
    const result = await response.json();
    $('#route-result').textContent = JSON.stringify(result, null, 2);
  } catch (error) {
    $('#route-result').textContent = error.message;
  }
});

loadStatus();
