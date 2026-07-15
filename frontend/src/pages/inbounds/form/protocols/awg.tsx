import { useState } from 'react';
import { useFormContext } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, message, Select, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { FormField } from '@/components/form/rhf';
import { HttpUtil, Wireguard } from '@/utils';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

// LUCX-HOOK: AWG — map the panel obfLevel (1/2/3) and mimicryProfile to the
// backend cps package's profile enums. The backend owns the invariant-
// enforcing RNG (Jmin<Jmax, |S1+56-S2|>=10, H1-H4 in disjoint quadrants) and
// the CPS packet generators (TLS/DNS/SIP/QUIC), so the form calls the API
// instead of a local Math.random stub.
const OBF_PROFILE: Record<number, string> = { 1: 'lite', 2: 'standard', 3: 'pro' };

function levelToFullI1I5(level: number): boolean {
  return level >= 3;
}

// generateAwgObfuscationFromBackend calls /panel/api/inbounds/awg/generateObfuscation
// to get a fresh Jc/Jmin/Jmax/S1-S4/H1-H4 + I1-I5 set from the server.
async function generateAwgObfuscationFromBackend(getValue: (name: string) => unknown): Promise<Record<string, unknown> | null> {
  const level = (getValue('settings.obfLevel') as number) ?? 2;
  const mimicryProfile = (getValue('settings.mimicryProfile') as string) || 'quic';
  const region = (getValue('settings.region') as string) || 'world';
  const obfProfile = OBF_PROFILE[level] ?? 'standard';
  const fullI1I5 = levelToFullI1I5(level);
  const msg = await HttpUtil.post('/panel/api/inbounds/awg/generateObfuscation', {
    obfProfile,
    mimicryProfile,
    region,
    domain: '',
    fullI1I5,
  }, { headers: { 'Content-Type': 'application/json' } });
  if (!msg?.success) return null;
  return (msg?.obj ?? null) as Record<string, unknown> | null;
}

// captureHostSignature captures a real QUIC handshake from the given domain.
async function captureHostSignature(domain: string): Promise<Record<string, string> | null> {
  const msg = await HttpUtil.post('/panel/api/inbounds/awg/captureHost', { domain }, { headers: { 'Content-Type': 'application/json' } });
  if (!msg?.success) return null;
  return (msg?.obj ?? null) as Record<string, string> | null;
}
// END LUCX-HOOK

export default function AwgFields() {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  // react-hook-form context (the inbound form is rhf, NOT AntD form). Use
  // useFormContext so setValue/getValue read/write the same store the
  // FormField/Controller bindings use — a plain AntD Form.useFormInstance()
  // would silently no-op against the rhf store (the root cause of the
  // "fields don't load/save" bug).
  const { setValue, watch } = useFormContext();
  const obfLevel = watch('settings.obfLevel') as number | undefined;
  const routeThroughXray = watch('settings.routeThroughXray') as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();

  const regenerateKeys = () => {
    const kp = Wireguard.generateKeypair();
    setValue('settings.privateKey', kp.privateKey, { shouldDirty: true });
    setValue('settings.publicKey', kp.publicKey, { shouldDirty: true });
  };

  // LUCX-HOOK: AWG — generate obfuscation via the backend (invariants + CPS).
  const [generating, setGenerating] = useState(false);
  const regenerateObfuscation = async () => {
    setGenerating(true);
    try {
      const obf = await generateAwgObfuscationFromBackend((name) => watch(name as never));
      if (!obf) {
        messageApi.error(t('pages.inbounds.form.awgRegenerateFailed'));
        return;
      }
      for (const [k, v] of Object.entries(obf)) {
        setValue(`settings.${k}`, v, { shouldDirty: true });
      }
      messageApi.success(t('pages.inbounds.form.awgRegenerateDone'));
    } finally {
      setGenerating(false);
    }
  };

  // LUCX-HOOK: AWG — capture a real QUIC handshake from a front domain.
  const [captureDomain, setCaptureDomain] = useState('');
  const [capturing, setCapturing] = useState(false);
  const captureHost = async () => {
    const dom = captureDomain.trim();
    if (!dom) {
      messageApi.error(t('pages.inbounds.form.awgCaptureDomainRequired'));
      return;
    }
    setCapturing(true);
    try {
      const res = await captureHostSignature(dom);
      if (!res || !res.i1) {
        messageApi.error(t('pages.inbounds.form.awgCaptureFailed'));
        return;
      }
      for (const k of ['i1', 'i2', 'i3', 'i4', 'i5']) {
        setValue(`settings.${k}`, (res as Record<string, string>)[k] ?? '', { shouldDirty: true });
      }
      messageApi.success(t('pages.inbounds.form.awgCaptureDone'));
    } finally {
      setCapturing(false);
    }
  };
  // END LUCX-HOOK

  return (
    <>
      {messageContextHolder}
      <Form.Item label={t('pages.inbounds.form.awgServerKeys')}>
        <Space.Compact block>
          <FormField name={['settings', 'privateKey']} noStyle>
            <Input readOnly placeholder={t('pages.inbounds.form.awgPrivateKey')} style={{ width: '50%' }} />
          </FormField>
          <FormField name={['settings', 'publicKey']} noStyle>
            <Input readOnly placeholder={t('pages.inbounds.form.awgPublicKey')} style={{ width: 'calc(50% - 32px)' }} />
          </FormField>
          <Button icon={<ReloadOutlined />} onClick={regenerateKeys} />
        </Space.Compact>
      </Form.Item>

      <FormField
        name={['settings', 'obfLevel']}
        label={t('pages.inbounds.form.awgObfLevel')}
        tooltip={t('pages.inbounds.form.awgObfLevelHint')}
      >
        <Select
          options={[
            { value: 1, label: 'Lite — лёгкая обфускация (Jc + DNS I1)' },
            { value: 2, label: 'Standard — средняя (Jc/S/H + TLS I1)' },
            { value: 3, label: 'Pro — полная (Jc/S/H + I1-I5)' },
          ]}
        />
      </FormField>

      <FormField name={['settings', 'mimicryProfile']} label={t('pages.inbounds.form.awgMimicryProfile')} tooltip={t('pages.inbounds.form.awgMimicryProfileHint')}>
        <Select
          options={[
            { value: 'tls', label: 'TLS (ClientHello, Chrome-like)' },
            { value: 'quic', label: 'QUIC (Initial packet)' },
            { value: 'dns', label: 'DNS (EDNS0 query)' },
            { value: 'sip', label: 'SIP (REGISTER)' },
          ]}
        />
      </FormField>

      <FormField name={['settings', 'region']} label={t('pages.inbounds.form.awgRegion')} tooltip={t('pages.inbounds.form.awgRegionHint')}>
        <Select
          options={[
            { value: 'ru', label: 'RU (включая РФ-сервисы: yandex/vk/gosuslugi)' },
            { value: 'world', label: 'World (только глобальные домены)' },
          ]}
        />
      </FormField>

      <FormField name={['settings', 'address']} label={t('pages.inbounds.form.awgAddress')} tooltip={t('pages.inbounds.form.awgAddressHint')}>
        <Input placeholder="10.8.0.1/24" />
      </FormField>

      <FormField name={['settings', 'mtu']} label={t('pages.inbounds.form.awgMtu')}>
        <InputNumber min={576} max={65535} style={{ width: '100%' }} />
      </FormField>

      <FormField name={['settings', 'dns']} label={t('pages.inbounds.form.awgDns')}>
        <Input placeholder="1.1.1.1, 1.0.0.1" />
      </FormField>

      <Form.Item label={t('pages.inbounds.form.awgObfuscation')}>
        <Button icon={<ReloadOutlined />} onClick={regenerateObfuscation} loading={generating}>
          {t('pages.inbounds.form.awgRegenerate')}
        </Button>
      </Form.Item>

      {/* LUCX-HOOK: AWG — host scan (QUIC capture → I1-I5, hoaxisr/awg-manager pattern) */}
      <Form.Item label={t('pages.inbounds.form.awgCaptureHost')} tooltip={t('pages.inbounds.form.awgCaptureHostHint')}>
        <Space.Compact style={{ display: 'flex' }}>
          <Input
            placeholder="google.com"
            value={captureDomain}
            onChange={(e) => setCaptureDomain(e.target.value)}
            style={{ flex: 1 }}
          />
          <Button onClick={captureHost} loading={capturing}>
            {t('pages.inbounds.form.awgCapture')}
          </Button>
        </Space.Compact>
      </Form.Item>
      {/* END LUCX-HOOK */}

      {(obfLevel ?? 2) >= 2 && (
        <>
          <FormField name={['settings', 'jc']} label="Jc">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'jmin']} label="Jmin">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'jmax']} label="Jmax">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 's1']} label="S1">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 's2']} label="S2">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 's3']} label="S3">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 's4']} label="S4">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'h1']} label="H1">
            <Input placeholder="100000-500000" />
          </FormField>
          <FormField name={['settings', 'h2']} label="H2">
            <Input placeholder="600000-900000" />
          </FormField>
          <FormField name={['settings', 'h3']} label="H3">
            <Input placeholder="1000000-1500000" />
          </FormField>
          <FormField name={['settings', 'h4']} label="H4">
            <Input placeholder="1600000-2000000" />
          </FormField>
        </>
      )}

      {obfLevel === 3 && (
        <>
          <FormField name={['settings', 'i1']} label="I1">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
          <FormField name={['settings', 'i2']} label="I2">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
          <FormField name={['settings', 'i3']} label="I3">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
          <FormField name={['settings', 'i4']} label="I4">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
          <FormField name={['settings', 'i5']} label="I5">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
        </>
      )}

      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.awgRouteThroughXray')}
        tooltip={t('pages.inbounds.form.awgRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <FormField
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.awgRouteOutbound')}
          tooltip={t('pages.inbounds.form.awgRouteOutboundHint')}
        >
          <Select
            allowClear
            showSearch
            placeholder={t('pages.inbounds.form.awgRouteOutboundPlaceholder')}
            options={(outboundTags ?? []).map((tag) => ({ value: tag, label: tag }))}
          />
        </FormField>
      )}
    </>
  );
}