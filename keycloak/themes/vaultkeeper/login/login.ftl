<#import "template.ftl" as layout>
<@layout.registrationLayout displayMessage=!messagesPerField.existsError('username','password') displayInfo=realm.password && realm.registrationAllowed && !registrationDisabled??; section>
    <#if section = "header">
        <h2 class="vk-form-title">Sign in</h2>
        <p class="vk-form-subtitle">Authenticate with your organizational credentials.</p>
    <#elseif section = "form">
        <#if realm.password>
            <form id="kc-form-login" onsubmit="login.disabled = true; return true;" action="${url.loginAction}" method="post" class="vk-form">
                <#if !usernameHidden??>
                    <div class="vk-field">
                        <label for="username" class="vk-label">
                            <#if !realm.loginWithEmailAllowed>${msg("username")}<#elseif !realm.registrationEmailAsUsername>${msg("usernameOrEmail")}<#else>${msg("email")}</#if>
                        </label>
                        <input tabindex="1" id="username" name="username" value="${(login.username!'')}"
                               type="text" autofocus autocomplete="username"
                               aria-invalid="<#if messagesPerField.existsError('username','password')>true</#if>"
                               class="vk-input <#if messagesPerField.existsError('username','password')>vk-input-error</#if>"
                               placeholder="<#if !realm.loginWithEmailAllowed>${msg("username")}<#elseif !realm.registrationEmailAsUsername>${msg("usernameOrEmail")}<#else>${msg("email")}</#if>" />
                        <#if messagesPerField.existsError('username','password')>
                            <span class="vk-field-error" aria-live="polite">
                                ${kcSanitize(messagesPerField.getFirstError('username','password'))?no_esc}
                            </span>
                        </#if>
                    </div>
                </#if>

                <div class="vk-field">
                    <label for="password" class="vk-label">${msg("password")}</label>
                    <input tabindex="2" id="password" name="password" type="password" autocomplete="current-password"
                           aria-invalid="<#if messagesPerField.existsError('username','password')>true</#if>"
                           class="vk-input <#if messagesPerField.existsError('username','password')>vk-input-error</#if>"
                           placeholder="${msg("password")}" />
                </div>

                <div class="vk-form-options">
                    <#if realm.rememberMe && !usernameHidden??>
                        <div class="vk-checkbox-group">
                            <input tabindex="3" id="rememberMe" name="rememberMe" type="checkbox"
                                   <#if login.rememberMe??>checked</#if>
                                   class="vk-checkbox" />
                            <label for="rememberMe" class="vk-checkbox-label">${msg("rememberMe")}</label>
                        </div>
                    </#if>
                    <#if realm.resetPasswordAllowed>
                        <a tabindex="5" href="${url.loginResetCredentialsUrl}" class="vk-link">${msg("doForgotPassword")}</a>
                    </#if>
                </div>

                <input type="hidden" id="id-hidden-input" name="credentialId" <#if auth.selectedCredential?has_content>value="${auth.selectedCredential}"</#if>/>

                <button tabindex="4" name="login" id="kc-login" type="submit" class="vk-btn-primary">
                    ${msg("doLogIn")}
                </button>
            </form>
        </#if>
    <#elseif section = "info">
        <#if realm.password && realm.registrationAllowed && !registrationDisabled??>
            <span class="vk-info-text">${msg("noAccount")} <a tabindex="6" href="${url.registrationUrl}" class="vk-link">${msg("doRegister")}</a></span>
        </#if>
    </#if>
</@layout.registrationLayout>
