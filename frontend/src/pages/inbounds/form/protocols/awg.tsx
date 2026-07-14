import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { App, Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { HttpUtil, Wireguard } from '@/utils';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

// LUCX-HOOK: AWG — map the panel obfLevel (1/2/3) and mimicryProfile to the
// backend cps package's profile enums. The backend owns the invariant-
// enforcing RNG (Jmin<Jmax, |S1+56-S2|>=10, H1-H4 in disjoint quadrants) and
// the CPS packet generators (TLS/DNS/SIP/QUIC), so the form calls the API
// instead of a local Math.random stub.
const OBF_PROFILE: Record<number, string> = { 1: 'lite', 2: 'standard', 3: 'pro' };

// obfLevel 1 = no CPS; 2 = I1 only; 3 = full I1-I5. Mirrors the form's
// existing Select options (1 — none, 2 — Jc/S/H, 3 — full + CPS).
function levelToFullI1I5(level: number): boolean {
  return level >= 3;
}

// generateAwgObfuscationFromBackend calls /panel/api/inbounds/awg/generateObfuscation
// to get a fresh Jc/Jmin/Jmax/S1-S4/H1-H4 + I1-I5 set from the server. Returns
// null on failure (the caller falls back to leaving the fields untouched).
async function generateAwgObfuscationFromBackend(form: ReturnType<typeof Form.useFormInstance>): Promise<Record<string, unknown> | null> {
  const level = (form.getFieldValue(['settings', 'obfLevel']) as number) ?? 2;
  const mimicryProfile = (form.getFieldValue(['settings', 'mimicryProfile']) as string) || 'quic';
  const region = (form.getFieldValue(['settings', 'region']) as string) || 'world';
  const obfProfile = OBF_PROFILE[level] ?? 'standard';
  const fullI1I5 = levelToFullI1I5(level);
  const msg = await HttpUtil.post('/panel/api/inbounds/awg/generateObfuscation', {
    obfProfile,
    mimicryProfile,
    region,
    domain: '',
    fullI1I5,
  });
  if (!msg?.success) return null;
  return (msg?.obj ?? null) as Record<string, unknown> | null;
}

// LUCX-HOOK: AWG — capture a real QUIC handshake from the given domain and
// use it as the I1-I5 CPS signature (hoaxisr/awg-manager pattern). The user
// enters a front host (e.g. google.com); the server sends a QUIC Initial,
// reads the replies, and returns the packet bytes as CPS strings.
async function captureHostSignature(domain: string): Promise<Record<string, string> | null> {
  const msg = await HttpUtil.post('/panel/api/inbounds/awg/captureHost', { domain });
  if (!msg?.success) return null;
  return (msg?.obj ?? null) as Record<string, string> | null;
}
// END LUCX-HOOK

export default function AwgFields() {
  const { t } = useTranslation();
  const { message: messageApi } = App.useApp();
  const form = Form.useFormInstance();
  const obfLevel = Form.useWatch(['settings', 'obfLevel'], form) as number | undefined;
  const routeThroughXray = Form.useWatch(['settings', 'routeThroughXray'], form) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();

  const regenerateKeys = () => {
    const kp = Wireguard.generateKeypair();
    const psk = Wireguard.generatePresharedKey();
    form.setFieldValue(['settings', 'privateKey'], kp.privateKey);
    form.setFieldValue(['settings', 'publicKey'], kp.publicKey);
    form.setFieldValue(['settings', 'presharedKey'], psk);
  };

  // LUCX-HOOK: AWG — generate obfuscation via the backend (invariants + CPS).
  const [generating, setGenerating] = useState(false);
  const regenerateObfuscation = async () => {
    setGenerating(true);
    try {
      const obf = await generateAwgObfuscationFromBackend(form);
      if (!obf) {
        messageApi.error(t('pages.inbounds.form.awgRegenerateFailed'));
        return;
      }
      form.setFieldsValue({ settings: obf });
      messageApi.success(t('pages.inbounds.form.awgRegenerateDone'));
    } finally {
      setGenerating(false);
    }
  };

  // LUCX-HOOK: AWG — capture a real QUIC handshake from a front domain and
  // fill I1-I5 with the captured packet bytes (hoaxisr/awg-manager pattern).
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
      form.setFieldsValue({ settings: { i1: res.i1, i2: res.i2, i3: res.i3, i4: res.i4, i5: res.i5 } });
      messageApi.success(t('pages.inbounds.form.awgCaptureDone'));
    } finally {
      setCapturing(false);
    }
  };
  // END LUCX-HOOK

  return (
    <>
      <Form.Item label={t('pages.inbounds.form.awgServerKeys')}>
        <Space.Compact block>
          <Form.Item name={['settings', 'privateKey']} noStyle>
            <Input readOnly placeholder={t('pages.inbounds.form.awgPrivateKey')} style={{ width: '50%' }} />
          </Form.Item>
          <Form.Item name={['settings', 'publicKey']} noStyle>
            <Input readOnly placeholder={t('pages.inbounds.form.awgPublicKey')} style={{ width: 'calc(50% - 32px)' }} />
          </Form.Item>
          <Button icon={<ReloadOutlined />} onClick={regenerateKeys} />
        </Space.Compact>
      </Form.Item>

      <Form.Item
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
      </Form.Item>

      <Form.Item name={['settings', 'mimicryProfile']} label={t('pages.inbounds.form.awgMimicryProfile')} tooltip={t('pages.inbounds.form.awgMimicryProfileHint')}>
        <Select
          options={[
            { value: 'tls', label: 'TLS (ClientHello, Chrome-like)' },
            { value: 'quic', label: 'QUIC (Initial packet)' },
            { value: 'dns', label: 'DNS (EDNS0 query)' },
            { value: 'sip', label: 'SIP (REGISTER)' },
          ]}
        />
      </Form.Item>

      <Form.Item name={['settings', 'region']} label={t('pages.inbounds.form.awgRegion')} tooltip={t('pages.inbounds.form.awgRegionHint')}>
        <Select
          options={[
            { value: 'ru', label: 'RU (включая РФ-сервисы: yandex/vk/gosuslugi)' },
            { value: 'world', label: 'World (только глобальные домены)' },
          ]}
        />
      </Form.Item>

      <Form.Item name={['settings', 'mtu']} label={t('pages.inbounds.form.awgMtu')}>
        <InputNumber min={576} max={65535} style={{ width: '100%' }} />
      </Form.Item>

      <Form.Item name={['settings', 'dns']} label={t('pages.inbounds.form.awgDns')}>
        <Input placeholder="1.1.1.1, 1.0.0.1" />
      </Form.Item>

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
          <Form.Item name={['settings', 'jc']} label="Jc">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 'jmin']} label="Jmin">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 'jmax']} label="Jmax">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 's1']} label="S1">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 's2']} label="S2">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 's3']} label="S3">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 's4']} label="S4">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 'h1']} label="H1">
            <Input placeholder="100000-500000" />
          </Form.Item>
          <Form.Item name={['settings', 'h2']} label="H2">
            <Input placeholder="600000-900000" />
          </Form.Item>
          <Form.Item name={['settings', 'h3']} label="H3">
            <Input placeholder="1000000-1500000" />
          </Form.Item>
          <Form.Item name={['settings', 'h4']} label="H4">
            <Input placeholder="1600000-2000000" />
          </Form.Item>
        </>
      )}

      {obfLevel === 3 && (
        <>
          <Form.Item name={['settings', 'i1']} label="I1">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
          <Form.Item name={['settings', 'i2']} label="I2">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
          <Form.Item name={['settings', 'i3']} label="I3">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
          <Form.Item name={['settings', 'i4']} label="I4">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
          <Form.Item name={['settings', 'i5']} label="I5">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
        </>
      )}

      <Form.Item
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.awgRouteThroughXray')}
        tooltip={t('pages.inbounds.form.awgRouteThroughXrayHint')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      {routeThroughXray && (
        <Form.Item
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
        </Form.Item>
      )}
    </>
  );
}