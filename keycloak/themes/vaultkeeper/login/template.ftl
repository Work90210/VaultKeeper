<#macro registrationLayout bodyClass="" displayInfo=false displayMessage=true displayRequiredFields=false>
<!DOCTYPE html>
<html lang="${locale.currentLanguageTag!'en'}">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="robots" content="noindex, nofollow">
    <title>${msg("loginTitle",(realm.displayName!''))}</title>
    <#if properties.meta?has_content>
        <#list properties.meta?split(' ') as meta>
            <meta name="${meta?split('==')[0]}" content="${meta?split('==')[1]}"/>
        </#list>
    </#if>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=DM+Serif+Display&family=Source+Sans+3:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <#if properties.styles?has_content>
        <#list properties.styles?split(' ') as style>
            <link href="${url.resourcesPath}/${style}" rel="stylesheet" />
        </#list>
    </#if>
</head>
<body class="vk-body">
    <div class="vk-layout">
        <!-- Left: brand panel -->
        <div class="vk-brand-panel">
            <div class="vk-brand-grid"></div>
            <div class="vk-brand-content">
                <div class="vk-brand-top">
                    <div class="vk-brand-logo">
                        <svg width="28" height="34" viewBox="0 0 22 26" fill="none" aria-hidden="true">
                            <path d="M11 1L2 5v7c0 6.075 3.75 10.35 9 12 5.25-1.65 9-5.925 9-12V5L11 1z" stroke="#b8954a" stroke-width="1.5" fill="none"/>
                            <path d="M11 7v6m0 2.5v.5" stroke="#b8954a" stroke-width="1.5" stroke-linecap="round"/>
                        </svg>
                        <h1 class="vk-brand-title">VaultKeeper</h1>
                    </div>
                    <p class="vk-brand-subtitle">Sovereign Evidence Management</p>
                </div>
                <div class="vk-brand-bottom">
                    <div class="vk-brand-quote">
                        <p>Tamper-evident chain of custody. Role-based access control. Cryptographic integrity verification for every piece of evidence.</p>
                    </div>
                    <div class="vk-brand-features">
                        <span class="vk-feature"><span class="vk-feature-dot"></span>Chain of Custody</span>
                        <span class="vk-feature"><span class="vk-feature-dot"></span>Cryptographic Integrity</span>
                        <span class="vk-feature"><span class="vk-feature-dot"></span>Full Audit Trail</span>
                    </div>
                    <p class="vk-brand-footer">All access is logged and auditable.</p>
                </div>
            </div>
        </div>

        <!-- Right: form panel -->
        <div class="vk-form-panel">
            <div class="vk-form-container">
                <!-- Mobile brand mark -->
                <div class="vk-mobile-brand">
                    <div class="vk-brand-logo">
                        <svg width="24" height="30" viewBox="0 0 22 26" fill="none" aria-hidden="true">
                            <path d="M11 1L2 5v7c0 6.075 3.75 10.35 9 12 5.25-1.65 9-5.925 9-12V5L11 1z" stroke="#b8954a" stroke-width="1.5" fill="none"/>
                            <path d="M11 7v6m0 2.5v.5" stroke="#b8954a" stroke-width="1.5" stroke-linecap="round"/>
                        </svg>
                        <h1 class="vk-brand-title" style="font-size: 1.5rem;">VaultKeeper</h1>
                    </div>
                    <p class="vk-brand-subtitle">Evidence Management</p>
                </div>

                <div class="vk-card">
                    <#if realm.internationalizationEnabled && locale.supported?size gt 1>
                        <div class="vk-locale-picker">
                            <#list locale.supported as l>
                                <a href="${l.url}" class="vk-locale-link <#if l.languageTag == locale.currentLanguageTag>vk-locale-active</#if>">${l.label}</a>
                            </#list>
                        </div>
                    </#if>

                    <#nested "header">

                    <#if displayMessage && message?has_content && (message.type != 'warning' || !isAppInitiatedAction??)>
                        <div class="vk-alert vk-alert-${message.type}">
                            ${kcSanitize(message.summary)?no_esc}
                        </div>
                    </#if>

                    <#nested "form">

                    <#if displayInfo>
                        <div class="vk-info">
                            <#nested "info">
                        </div>
                    </#if>
                </div>

                <p class="vk-restricted">Access restricted to authorized personnel</p>
            </div>
        </div>
    </div>
</body>
</html>
</#macro>
