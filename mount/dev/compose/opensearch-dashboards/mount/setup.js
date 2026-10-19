// Provision a useful first view without replacing saved objects or user settings.
const search = 'http://logs-opensearch:9200';
const dashboards = 'http://opensearch-dashboards:5601/proxy/logs';

async function request(url, method = 'GET', body, allowConflict = false) {
    const response = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json', 'osd-xsrf': 'mwcli' },
        body: body === undefined ? undefined : JSON.stringify(body),
        signal: AbortSignal.timeout(10000),
    });
    if (allowConflict && response.status === 409) {
        return;
    }
    if (!response.ok) {
        throw new Error(`${method} ${url}: ${response.status} ${await response.text()}`);
    }
    return response.json();
}

async function waitForDashboards() {
    for (let attempt = 0; attempt < 60; attempt++) {
        try {
            await request(`${dashboards}/api/status`);
            return;
        } catch (error) {
            if (attempt === 59) {
                throw error;
            }
            await new Promise(resolve => setTimeout(resolve, 2000));
        }
    }
}

async function main() {
    await waitForDashboards();
    await request(`${search}/_index_template/mwcli-logs`, 'PUT', {
        index_patterns: ['mwcli-logs-*'],
        template: {
            settings: { number_of_shards: 1, number_of_replicas: 0 },
            mappings: { properties: { '@timestamp': { type: 'date' } } },
        },
    });
    await request(`${dashboards}/api/saved_objects/index-pattern/mwcli-logs`, 'POST', {
        attributes: { title: 'mwcli-logs-*', timeFieldName: '@timestamp' },
    }, true);
    await request(`${dashboards}/api/saved_objects/search/mwcli-mediawiki-logs`, 'POST', {
        attributes: {
            title: 'MediaWiki logs',
            description: 'MediaWiki debug files, web requests, PHP output and jobrunner logs for this development context.',
            columns: ['service', 'source', 'stream', 'message'],
            sort: [['@timestamp', 'desc']],
            kibanaSavedObjectMeta: {
                searchSourceJSON: JSON.stringify({
                    indexRefName: 'kibanaSavedObjectMeta.searchSourceJSON.index',
                    query: {
                        language: 'lucene',
                        query: 'service:(mediawiki OR mediawiki-web OR mediawiki-jobrunner)',
                    },
                    filter: [],
                }),
            },
        },
        references: [{
            id: 'mwcli-logs', type: 'index-pattern',
            name: 'kibanaSavedObjectMeta.searchSourceJSON.index',
        }],
    }, true);
    const { settings } = await request(`${dashboards}/api/opensearch-dashboards/settings`);
    const defaults = {
        defaultIndex: 'mwcli-logs',
        defaultRoute: '/app/discover#/view/mwcli-mediawiki-logs',
    };
    const changes = Object.fromEntries(Object.entries(defaults)
        .filter(([key]) => settings[key]?.userValue === undefined));
    if (Object.keys(changes).length) {
        await request(`${dashboards}/api/opensearch-dashboards/settings`, 'POST', { changes });
    }
    console.log('MediaWiki log view is ready at /proxy/logs/app/discover#/view/mwcli-mediawiki-logs');
}

main().catch(error => {
    console.error(`Unable to prepare the MediaWiki log view: ${error.message}`);
    process.exitCode = 1;
});
