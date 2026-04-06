<#import "template.ftl" as layout>
<@layout.registrationLayout displayInfo=true displayMessage=!messagesPerField.existsError('username'); section>
    <#if section = "header">
        <h2 class="vk-form-title">${msg("emailForgotTitle")}</h2>
        <p class="vk-form-subtitle">${msg("emailInstruction")}</p>
    <#elseif section = "form">
        <form id="kc-reset-password-form" action="${url.loginAction}" method="post" class="vk-form">
            <div class="vk-field">
                <label for="username" class="vk-label">
                    <#if !realm.loginWithEmailAllowed>${msg("username")}<#elseif !realm.registrationEmailAsUsername>${msg("usernameOrEmail")}<#else>${msg("email")}</#if>
                </label>
                <input type="text" id="username" name="username" autofocus
                       class="vk-input <#if messagesPerField.existsError('username')>vk-input-error</#if>"
                       value="${(auth.attemptedUsername!'')}" />
                <#if messagesPerField.existsError('username')>
                    <span class="vk-field-error" aria-live="polite">
                        ${kcSanitize(messagesPerField.getFirstError('username'))?no_esc}
                    </span>
                </#if>
            </div>

            <div style="display: flex; gap: var(--space-md); align-items: center;">
                <button type="submit" class="vk-btn-primary" style="flex: 1;">${msg("doSubmit")}</button>
                <a href="${url.loginUrl}" class="vk-link">${kcSanitize(msg("backToLogin"))?no_esc}</a>
            </div>
        </form>
    <#elseif section = "info">
        <#-- info handled by subtitle above -->
    </#if>
</@layout.registrationLayout>
