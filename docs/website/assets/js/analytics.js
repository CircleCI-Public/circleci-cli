/**
 * Copyright (c) 2026 Circle Internet Services, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 *
 * SPDX-License-Identifier: MIT
 */

/* Segment page tracking. Injected by layouts/partials/analytics.html into a
   type="text/plain" data-cookieconsent="statistics" script tag, so Cookiebot
   runs this only after the visitor grants statistics consent — see that partial
   for the full rationale. Values are supplied by the partial's template
   execution and arrive here already quoted. */
if (
  /* Artifact previews and local mirrors of this build are not the tracked site. */
  location.hostname === {{ .host }} &&
  !navigator.webdriver &&
  !/bot|crawl|spider|slurp|headless|preview|monitor|scan|lighthouse/i.test(navigator.userAgent)
) {
  /* Segment analytics.js snippet v5.2.1, verbatim from the vendor. It opens a
     block that the load/page calls below sit inside; `}}();` closes it. */
  !function(){var i="analytics",analytics=window[i]=window[i]||[];if(!analytics.initialize)if(analytics.invoked)window.console&&console.error&&console.error("Segment snippet included twice.");else{analytics.invoked=!0;analytics.methods=["trackSubmit","trackClick","trackLink","trackForm","pageview","identify","reset","group","track","ready","alias","debug","page","screen","once","off","on","addSourceMiddleware","addIntegrationMiddleware","setAnonymousId","addDestinationMiddleware","register"];analytics.factory=function(e){return function(){if(window[i].initialized)return window[i][e].apply(window[i],arguments);var n=Array.prototype.slice.call(arguments);if(["track","screen","alias","group","page","identify"].indexOf(e)>-1){var c=document.querySelector("link[rel='canonical']");n.push({__t:"bpc",c:c&&c.getAttribute("href")||void 0,p:location.pathname,u:location.href,s:location.search,t:document.title,r:document.referrer})}n.unshift(e);analytics.push(n);return analytics}};for(var n=0;n<analytics.methods.length;n++){var key=analytics.methods[n];analytics[key]=analytics.factory(key)}analytics.load=function(key,n){var t=document.createElement("script");t.type="text/javascript";t.async=!0;t.setAttribute("data-global-segment-analytics-key",i);t.src="https://cdn.segment.com/analytics.js/v1/" + key + "/analytics.min.js";var r=document.getElementsByTagName("script")[0];r.parentNode.insertBefore(t,r);analytics._loadOptions=n};analytics._writeKey={{ .writeKey }};;analytics.SNIPPET_VERSION="5.2.1";

  /* Statistics consent alone must not release an advertising destination.
     Segment's core cascades its bundled device-mode destinations when it loads,
     and the ad destination is one of them, so withhold it unless marketing
     consent was granted too. Segment reads this once at load and cannot be
     reconfigured afterwards, so a later marketing grant applies on the next
     page load. */
  var marketingGranted = !!(window.Cookiebot && Cookiebot.consent && Cookiebot.consent.marketing);
  analytics.load({{ .writeKey }}, marketingGranted ? {} : { integrations: { AdWords: false } });

  /* Property names match the page events the web app sends, snake_cased, so
     both can be reported on with one set of definitions. */
  analytics.page({{ .pageName }}, {
    action: "viewed",
    page_type: "outer",
    page_name: {{ .pageName }},
    team_name: "webex",
    path: location.pathname,
    referrer: document.referrer,
    search: location.search,
    title: document.title,
    url: location.href,
    window_width: window.innerWidth,
    window_height: window.innerHeight
  });
  }}();
}
