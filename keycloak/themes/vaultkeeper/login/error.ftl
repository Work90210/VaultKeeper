<#import "template.ftl" as layout>
<@layout.registrationLayout displayMessage=false; section>
    <#if section = "header">
        <h2 class="vk-form-title">${msg("errorTitle")}</h2>
    <#elseif section = "form">
        <div class="vk-alert vk-alert-error">
            ${kcSanitize(message.summary)?no_esc}
        </div>
        <#if skipLink??>
        <#else>
            <#if client?? && client.baseUrl?has_content>
                <a href="${client.baseUrl}" class="vk-btn-primary" style="text-align: center; text-decoration: none; display: block; margin-top: var(--space-md);">${kcSanitize(msg("backToApplication"))?no_esc}</a>
            </#if>
        </#if>
    </#if>
</@layout.registrationLayout>
