package rod

const evasionScript = `
if (!window.chrome) {
    window.chrome = {};
}
if (!window.chrome.runtime) {
    window.chrome.runtime = {
        connect: function() {},
        sendMessage: function() {},
        onMessage: { addListener: function() {} },
        id: undefined
    };
}

if (!window.chrome.csi) {
    window.chrome.csi = function() {
        return {
            startE: Date.now(),
            onloadT: Date.now(),
            pageT: Math.random() * 1000 + 500,
            tran: 15
        };
    };
}

if (!window.chrome.loadTimes) {
    window.chrome.loadTimes = function() {
        return {
            commitLoadTime: Date.now() / 1000,
            connectionInfo: "h2",
            finishDocumentLoadTime: Date.now() / 1000,
            finishLoadTime: Date.now() / 1000,
            firstPaintAfterLoadTime: 0,
            firstPaintTime: Date.now() / 1000,
            navigationType: "Other",
            npnNegotiatedProtocol: "h2",
            requestTime: Date.now() / 1000 - 0.16,
            startLoadTime: Date.now() / 1000,
            wasAlternateProtocolAvailable: false,
            wasFetchedViaSpdy: true,
            wasNpnNegotiated: true
        };
    };
}

if (!navigator.connection) {
    Object.defineProperty(navigator, 'connection', {
        get: () => ({
            effectiveType: '4g',
            rtt: 50,
            downlink: 10,
            saveData: false
        }),
        configurable: true
    });
}

(function() {
    const origUA = navigator.userAgent;
    const origAV = navigator.appVersion;
    if (origUA.includes('HeadlessChrome')) {
        Object.defineProperty(navigator, 'userAgent', {
            get: () => origUA.replace('HeadlessChrome', 'Chrome'),
            configurable: true
        });
        Object.defineProperty(navigator, 'appVersion', {
            get: () => origAV.replace('HeadlessChrome', 'Chrome'),
            configurable: true
        });
    }
})();
`
