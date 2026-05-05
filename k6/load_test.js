import http from 'k6/http';
import { parseHTML } from 'k6/html';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: __ENV.RAMP_UP || '1m', target: parseInt(__ENV.VUS || '20') },
        { duration: __ENV.DURATION || '5m', target: parseInt(__ENV.VUS || '20') },
        { duration: __ENV.RAMP_DOWN || '1m', target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(95)<1000'], // Relaxing slightly for 1k user scale
        http_req_failed: ['rate<0.01'],   // Less than 1% error rate
    },
};

// Helper function to discover URLs from sitemap
function getUrlsFromSitemap() {
    const sitemapUrl = 'https://orthodoxpilgrimage.com/sitemap.xml';
    const res = http.get(sitemapUrl);
    
    if (res.status !== 200) {
        console.error(`Failed to fetch sitemap: ${res.status}`);
        return [];
    }

    const doc = parseHTML(res.body);
    const urls = [];
    
    // In k6, parseHTML works on XML too. 
    // Sitemap tags are <url><loc>...</loc></url>
    doc.find('loc').each((idx, el) => {
        urls.push(el.textContent());
    });

    return urls;
}

// Fetch URLs once at the start of the test (init stage)
// Note: In k6, 'setup' is better for this if we want to share data between VUs
export function setup() {
    const urls = getUrlsFromSitemap();
    console.log(`Discovered ${urls.length} URLs from sitemap`);
    return { urls: urls };
}

export default function (data) {
    const urls = data.urls;
    
    if (urls.length === 0) {
        console.error('No URLs discovered, skipping iteration');
        return;
    }

    // Pick a random URL from the discovered list
    const url = urls[Math.floor(Math.random() * urls.length)];
    
    const res = http.get(url, {
        tags: { name: 'SitemapURL' },
    });

    check(res, {
        'is status 200': (r) => r.status === 200,
        'body size > 0': (r) => r.body.length > 0,
    });

    // Simulate think time
    sleep(Math.random() * 3 + 1);
}
