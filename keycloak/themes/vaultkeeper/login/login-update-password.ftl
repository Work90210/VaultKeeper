<#import "template.ftl" as layout>
<@layout.registrationLayout displayMessage=!messagesPerField.existsError('password','password-confirm'); section>
    <#if section = "header">
        <h2 class="vk-form-title">${msg("updatePasswordTitle")}</h2>
        <p class="vk-form-subtitle">Choose a new password for your account.</p>
    <#elseif section = "form">
        <form id="kc-passwd-update-form" action="${url.loginAction}" method="post" class="vk-form">
            <input type="text" id="username" name="username" value="${username}" autocomplete="username" readonly="readonly" style="display:none;" />
            <input type="password" id="password" name="password" autocomplete="current-password" style="display:none;" />

            <div class="vk-field">
                <label for="password-new" class="vk-label">${msg("passwordNew")}</label>
                <input type="password" id="password-new" name="password-new" autofocus autocomplete="new-password"
                       class="vk-input <#if messagesPerField.existsError('password','password-confirm')>vk-input-error</#if>" />
                <#if messagesPerField.existsError('password')>
                    <span class="vk-field-error" aria-live="polite">
                        ${kcSanitize(messagesPerField.getFirstError('password'))?no_esc}
                    </span>
                </#if>
            </div>

            <div class="vk-field">
                <label for="password-confirm" class="vk-label">${msg("passwordConfirm")}</label>
                <input type="password" id="password-confirm" name="password-confirm" autocomplete="new-password"
                       class="vk-input <#if messagesPerField.existsError('password-confirm')>vk-input-error</#if>" />
                <#if messagesPerField.existsError('password-confirm')>
                    <span class="vk-field-error" aria-live="polite">
                        ${kcSanitize(messagesPerField.getFirstError('password-confirm'))?no_esc}
                    </span>
                </#if>
            </div>

            <div style="display: flex; gap: var(--space-md); align-items: center;">
                <#if isAppInitiatedAction??>
                    <button type="submit" class="vk-btn-primary" style="flex: 1;">${msg("doSubmit")}</button>
                    <button type="submit" name="cancel-aia" value="true" class="vk-link">${msg("doCancel")}</button>
                <#else>
                    <button type="submit" class="vk-btn-primary" style="flex: 1;">${msg("doSubmit")}</button>
                </#if>
            </div>
        </form>
    </#if>
</@layout.registrationLayout>
